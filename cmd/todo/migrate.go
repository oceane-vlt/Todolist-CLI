package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oceane-vlt/todolist/libs/clientauth"
	"github.com/oceane-vlt/todolist/libs/storage"
	"github.com/oceane-vlt/todolist/libs/ui"
	"github.com/spf13/cobra"
)

// migrateCmd transfers the user's local JSON todo lists into Postgres for their
// authenticated account (Phase 5 of docs/implementation-plan.md). It is a
// one-shot, idempotent command: re-running it skips lists already present in
// Postgres, and the source data.json is renamed to a .bak backup on success —
// never deleted — so no data can be lost.
//
// Unlike the day-to-day CLI commands, migrate talks to Postgres directly (it is
// the one operation that bridges the two backends), so it needs DATABASE_URL and
// the user identity from the stored credentials. A --dry-run flag previews the
// migration without writing anything.
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate your local todo lists (data.json) into the remote Postgres store",
	Long: `Transfer the todo lists stored locally in ~/.config/todolist/data.json into
the remote Postgres database for your authenticated account.

The command is idempotent: lists already present in the database are skipped, so
it is safe to run again after an interruption. On success the local data.json is
renamed to data.json.bak (never deleted).

Requirements:
  - You must be logged in (run "todo login" first) so the migration is scoped to
    your user account.
  - DATABASE_URL must point at the Postgres database (the same one the server uses).`,
	Run: func(cmd *cobra.Command, args []string) {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		runMigrate(dryRun)
	},
}

// runMigrate performs the migration flow: resolve the local data file and the
// user identity, connect to Postgres, migrate, then back up the JSON file.
func runMigrate(dryRun bool) {
	jsonPath, err := localDataFilePath()
	if err != nil {
		ui.Error(err.Error())
		return
	}
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		ui.Error(fmt.Sprintf("no local data file to migrate at %s", jsonPath))
		return
	}

	userID, email, err := authenticatedUserID()
	if err != nil {
		ui.Error(err.Error())
		ui.Info("Run " + ui.Command("todo login") + " first so the migration is scoped to your account.")
		return
	}

	connString := os.Getenv(storage.EnvDatabaseURL)
	if connString == "" {
		ui.Error(storage.EnvDatabaseURL + " is not set.")
		ui.Info("Set " + ui.Command(storage.EnvDatabaseURL+"=postgres://...") +
			" to the same database the server uses, then re-run the migration.")
		return
	}

	ctx := context.Background()
	pg, err := storage.NewPgStorePool(ctx, connString)
	if err != nil {
		ui.Error(err.Error())
		return
	}
	defer pg.Close()

	// Scope every write to the authenticated user via the same context seam the
	// server's auth interceptor uses (Phase 2/3).
	ctx = storage.WithUserID(ctx, userID)

	if dryRun {
		previewMigration(ctx, jsonPath, pg, email)
		return
	}

	result, err := storage.MigrateJSONToStore(ctx, jsonPath, pg)
	if err != nil {
		ui.Error(err.Error())
		ui.Info("Nothing was backed up; your local data.json is untouched. Fix the issue and re-run (the migration is idempotent).")
		return
	}

	reportMigration(result, email)

	backup, err := storage.BackupFile(jsonPath)
	if err != nil {
		ui.Error(err.Error())
		ui.Info("Migration succeeded but the local file could not be renamed; it is still present at " + jsonPath + ".")
		return
	}
	ui.Info("Local data file preserved as " + backup + " (not deleted).")
	ui.Info("Switch the server to Postgres with " + ui.Command(storage.EnvStorageBackend+"=postgres") + " once you have verified your data.")
}

// previewMigration prints what a real migration would do, without writing.
func previewMigration(ctx context.Context, jsonPath string, dst storage.Store, email string) {
	data, err := storage.ReadTodoData(jsonPath)
	if err != nil {
		ui.Error(err.Error())
		return
	}
	// Mirror MigrateJSONToStore exactly: match titles case-insensitively so the
	// preview reports the same create/skip decisions the real run would make, and
	// iterate in the same deterministic (sorted) order.
	existing := map[string]bool{}
	for _, ls := range dst.GetTodoListsTitles(ctx) {
		existing[strings.ToLower(ls.Title)] = true
	}
	titles := make([]string, 0, len(data.Lists))
	for title := range data.Lists {
		titles = append(titles, title)
	}
	sort.Strings(titles)
	ui.Header(fmt.Sprintf("Dry run — migration preview for %s", email))
	var toCreate, toSkip int
	for _, title := range titles {
		if existing[strings.ToLower(title)] {
			ui.Info("  skip   " + title + " (already in Postgres)")
			toSkip++
		} else {
			ui.Info(fmt.Sprintf("  create %s (%d items)", title, len(data.Lists[title])))
			toCreate++
		}
	}
	ui.Info(fmt.Sprintf("\nWould create %d list(s), skip %d. No changes made; data.json untouched.", toCreate, toSkip))
}

// reportMigration prints the outcome of a completed migration.
func reportMigration(result storage.MigrationResult, email string) {
	ui.Success(fmt.Sprintf("Migrated %d list(s) for %s (%d already present, skipped).",
		result.Created, email, result.Skipped))
	for _, t := range result.CreatedTitles {
		ui.Info("  migrated " + t)
	}
	for _, t := range result.SkippedTitles {
		ui.Info("  skipped  " + t + " (already in Postgres)")
	}
}

// authenticatedUserID loads the stored credentials and returns the user_id and
// email to scope the migration. It errors when the user is not logged in.
func authenticatedUserID() (userID, email string, err error) {
	path, err := clientauth.CredentialsPath()
	if err != nil {
		return "", "", err
	}
	creds, err := clientauth.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("you are not logged in")
		}
		return "", "", err
	}
	if creds.UserID == "" {
		return "", "", fmt.Errorf("stored credentials have no user id; please log in again")
	}
	return creds.UserID, creds.Email, nil
}

// localDataFilePath resolves ~/.config/todolist/data.json read-only (it never
// creates the file, unlike the server's bootstrap helper).
func localDataFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".config", "todolist", "data.json"), nil
}

func init() {
	migrateCmd.Flags().Bool("dry-run", false, "Preview the migration without writing anything")
	rootCmd.AddCommand(migrateCmd)
}
