package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	todo "github.com/oceane-vlt/todolist/proto"
)

// newTestJSONStore creates a JSONStore backed by a fresh empty data file in a
// temp dir, mirroring the on-disk format the server bootstraps with.
func newTestJSONStore(t *testing.T) *JSONStore {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	if err := os.WriteFile(path, []byte(`{"lists":{}}`), 0644); err != nil {
		t.Fatalf("failed to seed data file: %v", err)
	}
	return NewJSONStore(path)
}

// TestJSONStoreImplementsStore is a compile-time + runtime guard that JSONStore
// satisfies the Store interface.
func TestJSONStoreImplementsStore(t *testing.T) {
	var _ Store = NewJSONStore("ignored.json")
}

// TestJSONStoreCreateAndShow verifies the adapter delegates to the JSON backend
// and round-trips items through disk unchanged.
func TestJSONStoreCreateAndShow(t *testing.T) {
	ctx := context.Background()
	s := newTestJSONStore(t)

	items := []*todo.Item{
		{Title: "Task 1", Description: "First", Priority: "high", DueDate: "2026-01-01"},
		{Title: "Task 2", Description: "Second"},
	}

	if err := s.CreateTodoList(ctx, "work", items); err != nil {
		t.Fatalf("CreateTodoList() error: %v", err)
	}

	got, err := s.ShowTodoListItems(ctx, "work")
	if err != nil {
		t.Fatalf("ShowTodoListItems() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ShowTodoListItems() returned %d items, want 2", len(got))
	}
	if got[0].Title != "Task 1" || got[0].Priority != "high" || got[0].DueDate != "2026-01-01" {
		t.Errorf("first item mismatch: %+v", got[0])
	}
	if got[1].Title != "Task 2" {
		t.Errorf("second item mismatch: %+v", got[1])
	}
}

// TestJSONStoreCreateDuplicateFails verifies error propagation through the adapter.
func TestJSONStoreCreateDuplicateFails(t *testing.T) {
	ctx := context.Background()
	s := newTestJSONStore(t)

	if err := s.CreateTodoList(ctx, "work", nil); err != nil {
		t.Fatalf("first CreateTodoList() error: %v", err)
	}
	if err := s.CreateTodoList(ctx, "work", nil); err == nil {
		t.Errorf("expected error creating duplicate list, got nil")
	}
}

// TestJSONStoreGetTitlesAndDelete verifies list enumeration and deletion via the adapter.
func TestJSONStoreGetTitlesAndDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestJSONStore(t)

	if err := s.CreateTodoList(ctx, "work", []*todo.Item{{Title: "a"}}); err != nil {
		t.Fatalf("CreateTodoList(work) error: %v", err)
	}
	if err := s.CreateTodoList(ctx, "home", nil); err != nil {
		t.Fatalf("CreateTodoList(home) error: %v", err)
	}

	titles := s.GetTodoListsTitles(ctx)
	if len(titles) != 2 {
		t.Fatalf("GetTodoListsTitles() returned %d lists, want 2", len(titles))
	}

	if err := s.DeleteTodoList(ctx, []string{"home"}); err != nil {
		t.Fatalf("DeleteTodoList() error: %v", err)
	}
	titles = s.GetTodoListsTitles(ctx)
	if len(titles) != 1 || titles[0].Title != "work" {
		t.Errorf("after delete, got %+v, want only [work]", titles)
	}
}

// TestJSONStoreUpdateItem verifies item renaming via the adapter.
func TestJSONStoreUpdateItem(t *testing.T) {
	ctx := context.Background()
	s := newTestJSONStore(t)

	if err := s.CreateTodoList(ctx, "work", []*todo.Item{{Title: "old"}}); err != nil {
		t.Fatalf("CreateTodoList() error: %v", err)
	}
	if err := s.UpdateTodoListItemData(ctx, "work", 0, "new"); err != nil {
		t.Fatalf("UpdateTodoListItemData() error: %v", err)
	}

	got, err := s.ShowTodoListItems(ctx, "work")
	if err != nil {
		t.Fatalf("ShowTodoListItems() error: %v", err)
	}
	if len(got) != 1 || got[0].Title != "new" {
		t.Errorf("after rename, got %+v, want title=new", got)
	}
}
