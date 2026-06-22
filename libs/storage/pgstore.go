package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	todo "github.com/oceane-vlt/todolist/proto"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PlaceholderUserID is the fixed user identity used by PgStore during Phase 1
// (mono-user, no auth). Real per-user isolation is introduced in Phase 2 and
// real authentication in Phase 3 of docs/implementation-plan.md. This UUID must
// match the placeholder user seeded by migrations/0001_init.sql.
const PlaceholderUserID = "00000000-0000-0000-0000-000000000001"

// PgStore is the Postgres implementation of Store, backed by pgx. It mirrors the
// observable behaviour of JSONStore exactly so the two are interchangeable behind
// the Store interface (see the parity tests in pgstore_test.go).
//
// Phase 2 scope (docs/implementation-plan.md): every query is scoped to a
// per-request user_id read from the context via UserIDFromContext (WHERE
// user_id = $1, logical key (user_id, title)). When the context carries no
// identity, the store falls back to defaultUserID so the mono-user dev/test
// path (Phase 1) keeps working unchanged. In Phase 3 the auth interceptor feeds
// the real user_id into the context, with no signature churn here.
type PgStore struct {
	pool *pgxpool.Pool
	// defaultUserID is used when the context carries no user identity (dev /
	// mono-user fallback). It defaults to PlaceholderUserID.
	defaultUserID string
}

// compile-time assertion that PgStore satisfies Store.
var _ Store = (*PgStore)(nil)

// NewPgStore returns a PgStore using the given connection pool. Requests with no
// user identity in their context fall back to the Phase 1 placeholder user.
func NewPgStore(pool *pgxpool.Pool) *PgStore {
	return &PgStore{pool: pool, defaultUserID: PlaceholderUserID}
}

// userID resolves the user identity for a request: the context value when
// present (Phase 2+), otherwise the store's default (mono-user fallback).
func (s *PgStore) userID(ctx context.Context) string {
	if id, ok := UserIDFromContext(ctx); ok {
		return id
	}
	return s.defaultUserID
}

// NewPgStorePool opens a new pgx connection pool from a connection string
// (e.g. the value of DATABASE_URL) and returns a PgStore backed by it. The
// caller owns the pool and should Close() it via the returned PgStore.Close.
func NewPgStorePool(ctx context.Context, connString string) (*PgStore, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}
	return NewPgStore(pool), nil
}

// Close releases the underlying connection pool.
func (s *PgStore) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// EnsureUser provisions the authenticated user in the users table if absent
// (just-in-time provisioning, Phase 3 home auth). It is idempotent: a user that
// already exists is left untouched. This is required because lists.user_id has a
// NOT NULL foreign key to users(id); without provisioning, the first write by a
// real (non-placeholder) authenticated user would violate that constraint.
//
// EnsureUser is deliberately NOT part of the Store interface: it is a Postgres
// concern (the JSON backend has no users table), exposed via the UserProvisioner
// interface so the server can wire it only when the store is a PgStore.
//
// The id/email pair comes from the validated, signed JWT (sub + email claim),
// never from a client field. ON CONFLICT (id) DO NOTHING makes repeated calls
// cheap and avoids racing concurrent first requests.
func (s *PgStore) EnsureUser(ctx context.Context, userID, email string) error {
	if userID == "" {
		return fmt.Errorf("EnsureUser: empty user id")
	}
	if email == "" {
		return fmt.Errorf("EnsureUser: empty email for user %s", userID)
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO users (id, email) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING`,
		userID, email,
	)
	if err != nil {
		return fmt.Errorf("EnsureUser: %w", err)
	}
	return nil
}

// findListID resolves a list title to its id, case-insensitively (mirroring the
// JSON findListKey behaviour). It returns ("", nil) when no such list exists.
func (s *PgStore) findListID(ctx context.Context, q pgx.Tx, title string) (string, error) {
	var id string
	err := q.QueryRow(ctx,
		`SELECT id FROM lists WHERE user_id = $1 AND lower(title) = lower($2)`,
		s.userID(ctx), title,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *PgStore) CreateTodoList(ctx context.Context, title string, items []*todo.Item) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	existing, err := s.findListID(ctx, tx, title)
	if err != nil {
		return err
	}
	if existing != "" {
		// Mirrors JSONStore: duplicate (case-insensitive) is an error.
		return fmt.Errorf("a todo list named %s already exists", title)
	}

	var listID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO lists (user_id, title) VALUES ($1, $2) RETURNING id`,
		s.userID(ctx), title,
	).Scan(&listID); err != nil {
		return err
	}

	for i, item := range items {
		if err := insertItem(ctx, tx, listID, i, item); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *PgStore) GetTodoListsTitles(ctx context.Context) []*todo.ListSize {
	// Count only non-completed items, mirroring parseTodoListNames.
	rows, err := s.pool.Query(ctx,
		`SELECT l.title,
		        COUNT(i.id) FILTER (WHERE i.completed = false) AS size
		   FROM lists l
		   LEFT JOIN items i ON i.list_id = l.id
		  WHERE l.user_id = $1
		  GROUP BY l.title`,
		s.userID(ctx),
	)
	if err != nil {
		fmt.Println("Error querying lists:", err)
		return nil
	}
	defer rows.Close()

	res := []*todo.ListSize{}
	for rows.Next() {
		var title string
		var size int32
		if err := rows.Scan(&title, &size); err != nil {
			fmt.Println("Error scanning list row:", err)
			return nil
		}
		res = append(res, &todo.ListSize{Title: title, Size: size})
	}
	if rows.Err() != nil {
		fmt.Println("Error iterating list rows:", rows.Err())
		return nil
	}
	return res
}

func (s *PgStore) DeleteTodoList(ctx context.Context, titles []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// First validate every title exists (mirrors JSONStore: all-or-nothing).
	for _, title := range titles {
		id, err := s.findListID(ctx, tx, title)
		if err != nil {
			return err
		}
		if id == "" {
			available, err := s.availableLists(ctx, tx)
			if err != nil {
				return err
			}
			return fmt.Errorf("todo list \"%s\" does not exist. Available lists:\n%s", title, available)
		}
	}

	for _, title := range titles {
		if _, err := tx.Exec(ctx,
			`DELETE FROM lists WHERE user_id = $1 AND lower(title) = lower($2)`,
			s.userID(ctx), title,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *PgStore) ShowTodoListItems(ctx context.Context, title string) ([]*todo.Item, error) {
	var listID string
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM lists WHERE user_id = $1 AND lower(title) = lower($2)`,
		s.userID(ctx), title,
	).Scan(&listID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("todo list %s does not exist", title)
	}
	if err != nil {
		return nil, err
	}

	return s.itemsForList(ctx, s.pool, listID)
}

func (s *PgStore) DeleteTodoListItems(ctx context.Context, title string, indicesToDelete []int32) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	listID, err := s.findListID(ctx, tx, title)
	if err != nil {
		return err
	}
	if listID == "" {
		return fmt.Errorf("todo list %s does not exist", title)
	}

	items, err := s.itemsForList(ctx, tx, listID)
	if err != nil {
		return err
	}

	for _, idx := range indicesToDelete {
		if idx < 0 || idx >= int32(len(items)) {
			return fmt.Errorf("item index %d out of range for todo list %s", idx+1, title)
		}
	}

	// Mirrors JSONStore deleteItems: deleted items are not removed, they are
	// marked completed=true (the list length is preserved).
	for _, idx := range indicesToDelete {
		if _, err := tx.Exec(ctx,
			`UPDATE items SET completed = true WHERE list_id = $1 AND position = $2`,
			listID, idx,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *PgStore) UpdateTodoListData(ctx context.Context, title string, newItems []*todo.Item) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	listID, err := s.findListID(ctx, tx, title)
	if err != nil {
		return err
	}
	if listID == "" {
		return fmt.Errorf("list %s don't exist", title)
	}

	var nextPos int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(position)+1, 0) FROM items WHERE list_id = $1`,
		listID,
	).Scan(&nextPos); err != nil {
		return err
	}

	// Mirrors JSONStore updateData: only the Title is appended; other fields
	// default (empty/false), matching the JSON behaviour.
	for i, item := range newItems {
		appended := &todo.Item{Title: item.Title}
		if err := insertItem(ctx, tx, listID, nextPos+i, appended); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *PgStore) UpdateTodoListItemData(ctx context.Context, title string, itemIndex int32, newTitle string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	listID, err := s.findListID(ctx, tx, title)
	if err != nil {
		return err
	}
	if listID == "" {
		// Mirrors JSONStore: unknown list is a silent no-op (returns nil).
		return nil
	}

	var count int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM items WHERE list_id = $1`, listID,
	).Scan(&count); err != nil {
		return err
	}
	if int(itemIndex) < 0 || int(itemIndex) >= count {
		// Mirrors JSONStore: out-of-range index is a silent no-op.
		return nil
	}

	if _, err := tx.Exec(ctx,
		`UPDATE items SET title = $1 WHERE list_id = $2 AND position = $3`,
		newTitle, listID, itemIndex,
	); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// querier is the subset of pgx used for reads, satisfied by both *pgxpool.Pool
// and pgx.Tx, so itemsForList can run inside or outside a transaction.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// itemsForList returns the items of a list ordered by position (mirroring the
// ordered JSON array). It returns a non-nil empty slice for an empty list to
// match JSONStore.
func (s *PgStore) itemsForList(ctx context.Context, q querier, listID string) ([]*todo.Item, error) {
	rows, err := q.Query(ctx,
		`SELECT title, description, completed, due_date, priority
		   FROM items WHERE list_id = $1 ORDER BY position`,
		listID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := []*todo.Item{}
	for rows.Next() {
		var it todo.Item
		if err := rows.Scan(&it.Title, &it.Description, &it.Completed, &it.DueDate, &it.Priority); err != nil {
			return nil, err
		}
		res = append(res, &it)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return res, nil
}

// availableLists renders the bullet list of existing titles used in the
// DeleteTodoList error message (mirrors displayList).
func (s *PgStore) availableLists(ctx context.Context, q pgx.Tx) (string, error) {
	rows, err := q.Query(ctx, `SELECT title FROM lists WHERE user_id = $1`, s.userID(ctx))
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var b strings.Builder
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return "", err
		}
		b.WriteString("- " + title + "\n")
	}
	if rows.Err() != nil {
		return "", rows.Err()
	}
	return b.String(), nil
}

// insertItem inserts a single item at the given position, mapping the proto
// Item fields to the items columns (docs/target-architecture.md §2.2).
func insertItem(ctx context.Context, tx pgx.Tx, listID string, position int, item *todo.Item) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO items (list_id, position, title, description, completed, due_date, priority)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		listID, position, item.Title, item.Description, item.Completed, item.DueDate, item.Priority,
	)
	return err
}
