package storage

import (
	"context"
	"testing"

	todo "github.com/oceane-vlt/todolist/proto"
)

// TestItemUpdateIsEmpty pins the rule the storage backends rely on to skip a
// write entirely: an update with no field set changes nothing.
func TestItemUpdateIsEmpty(t *testing.T) {
	empty := ""

	tests := []struct {
		name   string
		update ItemUpdate
		want   bool
	}{
		{"no field set", ItemUpdate{}, true},
		{"title set", titleUpdate("x"), false},
		{"description set", descriptionUpdate("x"), false},
		// A pointer to "" is a real change (it clears the field), not an absence.
		{"description cleared", ItemUpdate{Description: &empty}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.update.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestJSONStorePartialItemUpdate is the core guarantee of field presence:
// editing one field must leave the other untouched.
func TestJSONStorePartialItemUpdate(t *testing.T) {
	ctx := context.Background()
	s := newTestJSONStore(t)

	if err := s.CreateTodoList(ctx, "work", []*todo.Item{
		{Title: "call plumber", Description: "leak under the sink"},
	}); err != nil {
		t.Fatalf("CreateTodoList() error: %v", err)
	}

	// Change only the title: the description must survive.
	if err := s.UpdateTodoListItemData(ctx, "work", 0, titleUpdate("call the plumber")); err != nil {
		t.Fatalf("UpdateTodoListItemData(title) error: %v", err)
	}
	got := firstItem(t, ctx, s, "work")
	if got.Title != "call the plumber" {
		t.Errorf("title = %q, want %q", got.Title, "call the plumber")
	}
	if got.Description != "leak under the sink" {
		t.Errorf("description = %q, want it untouched", got.Description)
	}

	// Change only the description: the title must survive.
	if err := s.UpdateTodoListItemData(ctx, "work", 0, descriptionUpdate("leak under the kitchen sink")); err != nil {
		t.Fatalf("UpdateTodoListItemData(description) error: %v", err)
	}
	got = firstItem(t, ctx, s, "work")
	if got.Title != "call the plumber" {
		t.Errorf("title = %q, want it untouched", got.Title)
	}
	if got.Description != "leak under the kitchen sink" {
		t.Errorf("description = %q, want it updated", got.Description)
	}
}

// TestJSONStoreClearDescription checks that an explicit empty string clears the
// description, as opposed to an absent field which leaves it alone.
func TestJSONStoreClearDescription(t *testing.T) {
	ctx := context.Background()
	s := newTestJSONStore(t)

	if err := s.CreateTodoList(ctx, "work", []*todo.Item{
		{Title: "task", Description: "some context"},
	}); err != nil {
		t.Fatalf("CreateTodoList() error: %v", err)
	}

	if err := s.UpdateTodoListItemData(ctx, "work", 0, descriptionUpdate("")); err != nil {
		t.Fatalf("UpdateTodoListItemData() error: %v", err)
	}
	if got := firstItem(t, ctx, s, "work"); got.Description != "" {
		t.Errorf("description = %q, want it cleared", got.Description)
	}
}

// TestJSONStoreAddKeepsDescription covers the append path, which used to drop
// the description on the floor.
func TestJSONStoreAddKeepsDescription(t *testing.T) {
	ctx := context.Background()
	s := newTestJSONStore(t)

	if err := s.CreateTodoList(ctx, "work", nil); err != nil {
		t.Fatalf("CreateTodoList() error: %v", err)
	}
	if err := s.UpdateTodoListData(ctx, "work", []*todo.Item{
		{Title: "buy bread", Description: "from the bakery on the corner"},
	}); err != nil {
		t.Fatalf("UpdateTodoListData() error: %v", err)
	}

	got := firstItem(t, ctx, s, "work")
	if got.Description != "from the bakery on the corner" {
		t.Errorf("description = %q, want it preserved on add", got.Description)
	}
}

// firstItem reads back the first item of a list, failing the test on any error.
func firstItem(t *testing.T, ctx context.Context, s Store, list string) *todo.Item {
	t.Helper()
	items, err := s.ShowTodoListItems(ctx, list)
	if err != nil {
		t.Fatalf("ShowTodoListItems() error: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("ShowTodoListItems() returned no items")
	}
	return items[0]
}
