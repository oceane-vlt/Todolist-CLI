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
	// UpdateTodoListItemData edits the item at itemIndex in the given list,
	// changing only the fields set in update.
	UpdateTodoListItemData(ctx context.Context, title string, itemIndex int32, update ItemUpdate) error
}

// ItemUpdate describes a partial edit of a single item: a nil field is left
// untouched, a non-nil field is written (a pointer to "" clears the value).
//
// Modelling the edit as a struct of optional fields — rather than one plain
// argument per column — is what lets a caller change a description without
// having to restate the title, which in turn is what makes the REST PATCH on
// /v1/lists/{title}/items a genuine partial update. It also keeps this seam
// stable when the remaining item fields (due date, priority) become editable:
// they are new fields here, not a new signature.
type ItemUpdate struct {
	// Title, when non-nil, is the item's new title.
	Title *string
	// Description, when non-nil, is the item's new description.
	Description *string
}

// IsEmpty reports whether the update would change nothing, letting callers skip
// the write (and the storage round-trip) entirely.
func (u ItemUpdate) IsEmpty() bool {
	return u.Title == nil && u.Description == nil
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

func (s *JSONStore) UpdateTodoListItemData(_ context.Context, title string, itemIndex int32, update ItemUpdate) error {
	return UpdateTodoListItemData(s.path, title, itemIndex, update)
}
