package storage

import (
	"context"
	"fmt"
	"os"
	"strings"

	todo "github.com/oceane-vlt/todolist/proto"
)

// MigrationResult summarises the outcome of MigrateJSONToStore so the caller
// (the `todo migrate` command, Phase 5 of docs/implementation-plan.md) can
// report exactly what happened.
type MigrationResult struct {
	// Created is the number of lists transferred from the JSON file into the
	// destination store during this run.
	Created int
	// Skipped is the number of lists already present in the destination store
	// and therefore left untouched. A non-zero value on a re-run is the expected
	// idempotent behaviour.
	Skipped int
	// SkippedTitles holds the titles that were skipped, in JSON order.
	SkippedTitles []string
	// CreatedTitles holds the titles that were created, in JSON order.
	CreatedTitles []string
}

// MigrateJSONToStore copies every list (and its items) from the local JSON file
// at jsonPath into dst, scoped to whatever identity dst resolves from ctx. It is
// idempotent: a list whose title already exists in dst (case-insensitively) is
// skipped rather than re-created, so the command is safe to re-run after an
// interruption. Idempotency is also guaranteed at the storage layer by the
// UNIQUE(user_id, title) constraint (docs/target-architecture.md §2.2).
//
// The source JSON file is only read, never modified or deleted; renaming it to a
// .bak backup is the caller's responsibility (BackupFile) and happens only after
// a successful migration.
func MigrateJSONToStore(ctx context.Context, jsonPath string, dst Store) (MigrationResult, error) {
	var result MigrationResult

	data, err := ReadTodoData(jsonPath)
	if err != nil {
		return result, fmt.Errorf("failed to read %s: %w", jsonPath, err)
	}

	// Build a case-insensitive set of titles already present in the destination
	// so existing lists are skipped (idempotent re-runs).
	existing := make(map[string]struct{})
	for _, ls := range dst.GetTodoListsTitles(ctx) {
		existing[strings.ToLower(ls.Title)] = struct{}{}
	}

	for _, title := range sortedTitles(data.Lists) {
		if _, ok := existing[strings.ToLower(title)]; ok {
			result.Skipped++
			result.SkippedTitles = append(result.SkippedTitles, title)
			continue
		}

		items := itemsFromJSON(data.Lists[title])
		if err := dst.CreateTodoList(ctx, title, items); err != nil {
			return result, fmt.Errorf("failed to migrate list %q: %w", title, err)
		}
		result.Created++
		result.CreatedTitles = append(result.CreatedTitles, title)
	}

	return result, nil
}

// itemsFromJSON converts the JSON TodoItem records of a list into proto Items,
// preserving every field (title, description, completed, due date, priority) and
// their order.
func itemsFromJSON(records []TodoItem) []*todo.Item {
	items := make([]*todo.Item, 0, len(records))
	for _, r := range records {
		items = append(items, &todo.Item{
			Title:       r.Title,
			Description: r.Description,
			Completed:   r.Completed,
			DueDate:     r.DueDate,
			Priority:    r.Priority,
		})
	}
	return items
}

// sortedTitles returns the map keys in a deterministic (lexicographic) order so
// the migration is reproducible run to run.
func sortedTitles(lists map[string][]TodoItem) []string {
	titles := make([]string, 0, len(lists))
	for title := range lists {
		titles = append(titles, title)
	}
	// Simple insertion sort keeps the dependency surface minimal and the list of
	// todo lists is tiny.
	for i := 1; i < len(titles); i++ {
		for j := i; j > 0 && titles[j-1] > titles[j]; j-- {
			titles[j-1], titles[j] = titles[j], titles[j-1]
		}
	}
	return titles
}

// BackupSuffix is appended to the JSON data file once it has been migrated.
const BackupSuffix = ".bak"

// BackupFile renames the file at path to path+BackupSuffix, preserving the
// original data instead of deleting it (docs/implementation-plan.md §4: the JSON
// file is never destroyed during the transition). It returns the backup path.
// If a backup already exists it is left in place and a numbered suffix is used
// so an earlier backup is never overwritten.
func BackupFile(path string) (string, error) {
	backup := path + BackupSuffix
	if _, err := os.Stat(backup); err == nil {
		// A previous backup exists; pick the first free numbered suffix so we
		// never clobber earlier data.
		for n := 1; ; n++ {
			candidate := fmt.Sprintf("%s.%d", backup, n)
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				backup = candidate
				break
			} else if err != nil {
				return "", err
			}
		}
	}
	if err := os.Rename(path, backup); err != nil {
		return "", fmt.Errorf("failed to back up %s: %w", path, err)
	}
	return backup, nil
}
