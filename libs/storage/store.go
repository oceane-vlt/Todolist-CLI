package storage

import (
	"context"

	todo "github.com/oceane-vlt/todolist/proto"
)

// Store abstracts the persistence layer behind a stable contract so the
// underlying backend (local JSON today, Postgres later) can be swapped without
// touching the gRPC handlers. There is exactly one method per existing business
// RPC. A context.Context is threaded through every operation so later phases
// (per-user scoping, timeouts, cancellation) can rely on it without further
// signature churn; the current JSON implementation ignores it.
//
// This interface is the seam introduced in Phase 0 of docs/implementation-plan.md.
// It captures the current JSON behaviour exactly — no functional change.
type Store interface {
	// CreateTodoList creates a new list with the given title and initial items.
	CreateTodoList(ctx context.Context, title string, items []*todo.Item) error
	// GetTodoListsTitles returns the existing lists with their (non-completed) sizes.
	GetTodoListsTitles(ctx context.Context) []*todo.ListSize
	// DeleteTodoList removes the lists identified by the given titles.
	DeleteTodoList(ctx context.Context, titles []string) error
	// ShowTodoListItems returns the items of the given list.
	ShowTodoListItems(ctx context.Context, title string) ([]*todo.Item, error)
	// DeleteTodoListItems removes the items at the given indices from the list.
	DeleteTodoListItems(ctx context.Context, title string, indicesToDelete []int32) error
	// UpdateTodoListData appends the given items to the list.
	UpdateTodoListData(ctx context.Context, title string, newItems []*todo.Item) error
	// UpdateTodoListItemData renames the item at itemIndex in the given list.
	UpdateTodoListItemData(ctx context.Context, title string, itemIndex int32, newTitle string) error
}

// JSONStore is the local single-file JSON implementation of Store. It is a thin
// adapter over the existing package-level functions, preserving their behaviour
// exactly (whole-file rewrite of data.json). The context is accepted for
// interface compatibility but not used by the JSON backend.
type JSONStore struct {
	// path is the location of the JSON data file (e.g. ~/.config/todolist/data.json).
	path string
}

// NewJSONStore returns a JSONStore persisting to the given file path.
func NewJSONStore(path string) *JSONStore {
	return &JSONStore{path: path}
}

// compile-time assertion that JSONStore satisfies Store.
var _ Store = (*JSONStore)(nil)

func (s *JSONStore) CreateTodoList(_ context.Context, title string, items []*todo.Item) error {
	return CreateTodoList(s.path, title, items)
}

func (s *JSONStore) GetTodoListsTitles(_ context.Context) []*todo.ListSize {
	return GetTodoListsTitles(s.path)
}

func (s *JSONStore) DeleteTodoList(_ context.Context, titles []string) error {
	return DeleteTodoList(s.path, titles)
}

func (s *JSONStore) ShowTodoListItems(_ context.Context, title string) ([]*todo.Item, error) {
	return ShowTodoListItems(s.path, title)
}

func (s *JSONStore) DeleteTodoListItems(_ context.Context, title string, indicesToDelete []int32) error {
	return DeleteTodoListItems(s.path, title, indicesToDelete)
}

func (s *JSONStore) UpdateTodoListData(_ context.Context, title string, newItems []*todo.Item) error {
	return UpdateTodoListData(s.path, title, newItems)
}

func (s *JSONStore) UpdateTodoListItemData(_ context.Context, title string, itemIndex int32, newTitle string) error {
	return UpdateTodoListItemData(s.path, title, itemIndex, newTitle)
}
