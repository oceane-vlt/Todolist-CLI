package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// seedDataFile writes a data.json with the given JSON body and returns its path.
func seedDataFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("seed data file: %v", err)
	}
	return path
}

const sampleDataJSON = `{"lists":{
  "work":[
    {"title":"ship","description":"v1","completed":false,"dueDate":"2026-07-01","priority":"high"},
    {"title":"review","description":"","completed":true,"dueDate":"","priority":"low"}
  ],
  "home":[
    {"title":"groceries","description":"milk","completed":false,"dueDate":"","priority":""}
  ]
}}`

// TestMigrateJSONToStore checks a full migration into a JSONStore destination
// (no database required): every list and item is transferred, fields preserved.
func TestMigrateJSONToStore(t *testing.T) {
	ctx := context.Background()
	jsonPath := seedDataFile(t, sampleDataJSON)
	dst := newTestJSONStore(t)

	result, err := MigrateJSONToStore(ctx, jsonPath, dst)
	if err != nil {
		t.Fatalf("MigrateJSONToStore() error: %v", err)
	}
	if result.Created != 2 || result.Skipped != 0 {
		t.Fatalf("got created=%d skipped=%d, want created=2 skipped=0", result.Created, result.Skipped)
	}

	got := dst.GetTodoListsTitles(ctx)
	if len(got) != 2 {
		t.Fatalf("expected 2 lists migrated, got %d", len(got))
	}

	items, err := dst.ShowTodoListItems(ctx, "work")
	if err != nil {
		t.Fatalf("ShowTodoListItems(work): %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items in work, got %d", len(items))
	}
	// Field preservation on the first item.
	if items[0].Title != "ship" || items[0].Description != "v1" ||
		items[0].DueDate != "2026-07-01" || items[0].Priority != "high" || items[0].Completed {
		t.Fatalf("item fields not preserved: %+v", items[0])
	}
	// Completed flag preserved on the second item.
	if !items[1].Completed {
		t.Fatalf("expected second item completed=true, got %+v", items[1])
	}
}

// TestMigrateJSONToStoreIdempotent verifies a second run creates nothing and
// skips the already-present lists (the safe-to-re-run guarantee).
func TestMigrateJSONToStoreIdempotent(t *testing.T) {
	ctx := context.Background()
	jsonPath := seedDataFile(t, sampleDataJSON)
	dst := newTestJSONStore(t)

	if _, err := MigrateJSONToStore(ctx, jsonPath, dst); err != nil {
		t.Fatalf("first migration: %v", err)
	}

	result, err := MigrateJSONToStore(ctx, jsonPath, dst)
	if err != nil {
		t.Fatalf("second migration should not error, got: %v", err)
	}
	if result.Created != 0 || result.Skipped != 2 {
		t.Fatalf("re-run: got created=%d skipped=%d, want created=0 skipped=2", result.Created, result.Skipped)
	}
}

// TestMigrateJSONToStorePartial verifies that when one list already exists, only
// the missing list is created and the existing one is skipped.
func TestMigrateJSONToStorePartial(t *testing.T) {
	ctx := context.Background()
	jsonPath := seedDataFile(t, sampleDataJSON)
	dst := newTestJSONStore(t)

	// Pre-create "work" so the migration must skip it (case-insensitive).
	if err := dst.CreateTodoList(ctx, "WORK", nil); err != nil {
		t.Fatalf("pre-create: %v", err)
	}

	result, err := MigrateJSONToStore(ctx, jsonPath, dst)
	if err != nil {
		t.Fatalf("migration: %v", err)
	}
	if result.Created != 1 || result.Skipped != 1 {
		t.Fatalf("got created=%d skipped=%d, want created=1 skipped=1", result.Created, result.Skipped)
	}
	if len(result.CreatedTitles) != 1 || result.CreatedTitles[0] != "home" {
		t.Fatalf("expected only 'home' created, got %v", result.CreatedTitles)
	}
}

// TestBackupFile verifies the data file is renamed to .bak, never deleted, and
// that an existing backup is not overwritten.
func TestBackupFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	if err := os.WriteFile(path, []byte(`{"lists":{}}`), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	backup, err := BackupFile(path)
	if err != nil {
		t.Fatalf("BackupFile() error: %v", err)
	}
	if backup != path+BackupSuffix {
		t.Fatalf("backup path = %q, want %q", backup, path+BackupSuffix)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("original file should have been renamed away")
	}

	// A second backup must not clobber the first.
	if err := os.WriteFile(path, []byte(`{"lists":{}}`), 0644); err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	backup2, err := BackupFile(path)
	if err != nil {
		t.Fatalf("second BackupFile() error: %v", err)
	}
	if backup2 == backup {
		t.Fatalf("second backup overwrote the first: %q", backup2)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("first backup was clobbered: %v", err)
	}
}

// TestMigrateJSONToPgStore is an integration test (skips without a test DB) that
// migrates into a real PgStore scoped to a user_id and checks idempotency.
func TestMigrateJSONToPgStore(t *testing.T) {
	pg := newTestPgStore(t) // skips if no test DB
	ctx := WithUserID(context.Background(), PlaceholderUserID)
	jsonPath := seedDataFile(t, sampleDataJSON)

	result, err := MigrateJSONToStore(ctx, jsonPath, pg)
	if err != nil {
		t.Fatalf("migration into PgStore: %v", err)
	}
	if result.Created != 2 {
		t.Fatalf("expected 2 lists created, got %d", result.Created)
	}

	items, err := pg.ShowTodoListItems(ctx, "work")
	if err != nil {
		t.Fatalf("ShowTodoListItems: %v", err)
	}
	if len(items) != 2 || items[0].Title != "ship" || items[0].Priority != "high" {
		t.Fatalf("migrated items not as expected: %+v", items)
	}

	// Idempotent re-run.
	result2, err := MigrateJSONToStore(ctx, jsonPath, pg)
	if err != nil {
		t.Fatalf("re-run into PgStore: %v", err)
	}
	if result2.Created != 0 || result2.Skipped != 2 {
		t.Fatalf("re-run: got created=%d skipped=%d, want created=0 skipped=2", result2.Created, result2.Skipped)
	}
}
