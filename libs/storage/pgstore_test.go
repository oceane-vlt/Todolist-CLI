package storage

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	todo "github.com/oceane-vlt/todolist/proto"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EnvTestDatabaseURL points to a disposable Postgres instance for the PgStore
// integration/parity tests. When unset, the Postgres tests are skipped so the
// suite stays green without Docker (e.g. `make test` on a fresh checkout).
const EnvTestDatabaseURL = "TODO_TEST_DATABASE_URL"

// newTestPgStore connects to the test database, applies the schema, and truncates
// all data so each test starts from a clean state. It skips the test entirely
// when TODO_TEST_DATABASE_URL is not set.
func newTestPgStore(t *testing.T) *PgStore {
	t.Helper()

	connString := os.Getenv(EnvTestDatabaseURL)
	if connString == "" {
		t.Skipf("%s not set; skipping Postgres integration test (run `make db-up` then export %s)", EnvTestDatabaseURL, EnvTestDatabaseURL)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)

	applyMigration(t, ctx, pool)

	// Clean slate for this test (keep the placeholder user row).
	if _, err := pool.Exec(ctx, `TRUNCATE lists, items RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	return NewPgStore(pool)
}

// applyMigration loads migrations/0001_init.sql and executes it. The DDL uses
// IF NOT EXISTS / ON CONFLICT so it is safe to run repeatedly.
func applyMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	path := filepath.Join("..", "..", "migrations", "0001_init.sql")
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration %s: %v", path, err)
	}
	if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
}

// (newTestJSONStore is defined in store_test.go and reused here.)

func sampleItems() []*todo.Item {
	return []*todo.Item{
		{Title: "buy milk", Description: "2L", Completed: false, DueDate: "2026-07-01", Priority: "high"},
		{Title: "call bob", Description: "", Completed: false, DueDate: "", Priority: "low"},
	}
}

// sortedSizes returns list sizes sorted by title for order-independent compare
// (GetTodoListsTitles iterates a map for JSON and a query for PG; neither
// guarantees ordering).
func sortedSizes(in []*todo.ListSize) []*todo.ListSize {
	out := append([]*todo.ListSize(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

func sizesEqual(a, b []*todo.ListSize) bool {
	a, b = sortedSizes(a), sortedSizes(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Title != b[i].Title || a[i].Size != b[i].Size {
			return false
		}
	}
	return true
}

func itemsEqual(a, b []*todo.Item) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Title != b[i].Title ||
			a[i].Description != b[i].Description ||
			a[i].Completed != b[i].Completed ||
			a[i].DueDate != b[i].DueDate ||
			a[i].Priority != b[i].Priority {
			return false
		}
	}
	return true
}

// TestPgStoreImplementsStore is a compile + runtime guard that PgStore is a Store.
func TestPgStoreImplementsStore(t *testing.T) {
	var _ Store = (*PgStore)(nil)
}

// TestPgStoreParity runs the same sequence of operations against both backends
// and asserts identical observable results. This is the Phase 1 parity test
// required by docs/implementation-plan.md.
func TestPgStoreParity(t *testing.T) {
	pg := newTestPgStore(t) // skips if no test DB
	js := newTestJSONStore(t)
	ctx := context.Background()

	stores := map[string]Store{"json": js, "postgres": pg}

	// runBoth executes fn against both stores and reports per-backend errors.
	type result struct {
		items []*todo.Item
		sizes []*todo.ListSize
		err   error
	}
	run := func(t *testing.T, name string, fn func(s Store) result) (jsRes, pgRes result) {
		jsRes = fn(stores["json"])
		pgRes = fn(stores["postgres"])
		if (jsRes.err == nil) != (pgRes.err == nil) {
			t.Fatalf("%s: error parity mismatch: json err=%v, pg err=%v", name, jsRes.err, pgRes.err)
		}
		return jsRes, pgRes
	}

	// 1. Create lists.
	run(t, "create work", func(s Store) result {
		return result{err: s.CreateTodoList(ctx, "work", sampleItems())}
	})
	run(t, "create perso", func(s Store) result {
		return result{err: s.CreateTodoList(ctx, "perso", nil)}
	})

	// 2. Duplicate create (case-insensitive) must error on both.
	js2, pg2 := run(t, "duplicate", func(s Store) result {
		return result{err: s.CreateTodoList(ctx, "WORK", nil)}
	})
	if js2.err == nil || pg2.err == nil {
		t.Fatalf("expected duplicate-create error on both backends")
	}

	// 3. Append items via UpdateTodoListData (title-only append).
	run(t, "append", func(s Store) result {
		return result{err: s.UpdateTodoListData(ctx, "perso", []*todo.Item{{Title: "extra", Description: "ignored", Priority: "high"}})}
	})

	// 4. Show items parity.
	jsShow, pgShow := run(t, "show work", func(s Store) result {
		items, err := s.ShowTodoListItems(ctx, "work")
		return result{items: items, err: err}
	})
	if !itemsEqual(jsShow.items, pgShow.items) {
		t.Fatalf("show parity mismatch:\njson=%v\npg=%v", jsShow.items, pgShow.items)
	}

	// 5. Rename an item.
	run(t, "rename", func(s Store) result {
		return result{err: s.UpdateTodoListItemData(ctx, "work", 0, "buy oat milk")}
	})
	jsShow, pgShow = run(t, "show after rename", func(s Store) result {
		items, err := s.ShowTodoListItems(ctx, "work")
		return result{items: items, err: err}
	})
	if !itemsEqual(jsShow.items, pgShow.items) {
		t.Fatalf("rename parity mismatch:\njson=%v\npg=%v", jsShow.items, pgShow.items)
	}

	// 6. Delete (mark completed) item 1, then check sizes (non-completed count).
	run(t, "delete item", func(s Store) result {
		return result{err: s.DeleteTodoListItems(ctx, "work", []int32{1})}
	})
	jsSizes := stores["json"].GetTodoListsTitles(ctx)
	pgSizes := stores["postgres"].GetTodoListsTitles(ctx)
	if !sizesEqual(jsSizes, pgSizes) {
		t.Fatalf("sizes parity mismatch:\njson=%v\npg=%v", jsSizes, pgSizes)
	}

	// 7. Item still present (marked completed, not removed) on both.
	jsShow, pgShow = run(t, "show after delete", func(s Store) result {
		items, err := s.ShowTodoListItems(ctx, "work")
		return result{items: items, err: err}
	})
	if !itemsEqual(jsShow.items, pgShow.items) {
		t.Fatalf("post-delete parity mismatch:\njson=%v\npg=%v", jsShow.items, pgShow.items)
	}
	if len(pgShow.items) != 2 || !pgShow.items[1].Completed {
		t.Fatalf("expected deleted item kept and marked completed, got %v", pgShow.items)
	}

	// 8. Delete a whole list.
	run(t, "delete list", func(s Store) result {
		return result{err: s.DeleteTodoList(ctx, []string{"perso"})}
	})
	jsSizes = stores["json"].GetTodoListsTitles(ctx)
	pgSizes = stores["postgres"].GetTodoListsTitles(ctx)
	if !sizesEqual(jsSizes, pgSizes) {
		t.Fatalf("post-delete-list sizes mismatch:\njson=%v\npg=%v", jsSizes, pgSizes)
	}

	// 9. Delete unknown list errors on both.
	jsDel, pgDel := run(t, "delete unknown", func(s Store) result {
		return result{err: s.DeleteTodoList(ctx, []string{"ghost"})}
	})
	if jsDel.err == nil || pgDel.err == nil {
		t.Fatalf("expected delete-unknown error on both backends")
	}
}

// secondTestUserID is a distinct, seeded user used by the Phase 2 isolation
// test. It must exist in the users table (FK from lists.user_id), so the test
// inserts it explicitly.
const secondTestUserID = "00000000-0000-0000-0000-000000000002"

// TestPgStoreUserIsolation verifies Phase 2 multi-user isolation: two distinct
// user_id values (carried in the context) do not see each other's lists, and
// the same title is allowed for two different users (UNIQUE(user_id, title)).
// It covers every read RPC to guard against a missing user_id filter.
func TestPgStoreUserIsolation(t *testing.T) {
	pg := newTestPgStore(t) // skips if no test DB
	ctx := context.Background()

	// Seed a second user (placeholder user is already seeded by the migration).
	if _, err := pg.pool.Exec(ctx,
		`INSERT INTO users (id, email) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING`,
		secondTestUserID, "user2@example.com",
	); err != nil {
		t.Fatalf("seed second user: %v", err)
	}

	ctxA := WithUserID(ctx, PlaceholderUserID)
	ctxB := WithUserID(ctx, secondTestUserID)

	// Both users create a list with the SAME title (allowed per user).
	if err := pg.CreateTodoList(ctxA, "shared", []*todo.Item{{Title: "a-item"}}); err != nil {
		t.Fatalf("user A create: %v", err)
	}
	if err := pg.CreateTodoList(ctxB, "shared", []*todo.Item{{Title: "b-item-1"}, {Title: "b-item-2"}}); err != nil {
		t.Fatalf("user B create same title: %v", err)
	}

	// User A also creates a private list.
	if err := pg.CreateTodoList(ctxA, "a-only", nil); err != nil {
		t.Fatalf("user A create private: %v", err)
	}

	// GetTodoListsTitles: each user sees only their own lists.
	aLists := pg.GetTodoListsTitles(ctxA)
	if len(aLists) != 2 {
		t.Fatalf("user A should see 2 lists, got %v", aLists)
	}
	bLists := pg.GetTodoListsTitles(ctxB)
	if len(bLists) != 1 || bLists[0].Title != "shared" {
		t.Fatalf("user B should see only [shared], got %v", bLists)
	}

	// ShowTodoListItems: same title returns each user's own items.
	aItems, err := pg.ShowTodoListItems(ctxA, "shared")
	if err != nil {
		t.Fatalf("user A show: %v", err)
	}
	if len(aItems) != 1 || aItems[0].Title != "a-item" {
		t.Fatalf("user A items leaked/wrong: %v", aItems)
	}
	bItems, err := pg.ShowTodoListItems(ctxB, "shared")
	if err != nil {
		t.Fatalf("user B show: %v", err)
	}
	if len(bItems) != 2 || bItems[0].Title != "b-item-1" {
		t.Fatalf("user B items leaked/wrong: %v", bItems)
	}

	// User B cannot see user A's private list.
	if _, err := pg.ShowTodoListItems(ctxB, "a-only"); err == nil {
		t.Fatalf("user B should not see user A's private list")
	}

	// DeleteTodoListItems on B's "shared" must not affect A's "shared".
	if err := pg.DeleteTodoListItems(ctxB, "shared", []int32{0}); err != nil {
		t.Fatalf("user B delete item: %v", err)
	}
	aAfter, err := pg.ShowTodoListItems(ctxA, "shared")
	if err != nil {
		t.Fatalf("user A show after B delete: %v", err)
	}
	if len(aAfter) != 1 || aAfter[0].Completed {
		t.Fatalf("user B's delete leaked into user A's list: %v", aAfter)
	}

	// User B cannot delete user A's private list (scoped to B, not found).
	if err := pg.DeleteTodoList(ctxB, []string{"a-only"}); err == nil {
		t.Fatalf("user B should not be able to delete user A's list")
	}
	// User A's private list is still there.
	if _, err := pg.ShowTodoListItems(ctxA, "a-only"); err != nil {
		t.Fatalf("user A's private list should still exist, got %v", err)
	}
}

// TestPgStoreOutOfRangeIndex verifies the out-of-range index error and the
// silent no-ops match the JSON contract.
func TestPgStoreOutOfRangeIndex(t *testing.T) {
	pg := newTestPgStore(t) // skips if no test DB
	ctx := context.Background()

	if err := pg.CreateTodoList(ctx, "work", sampleItems()); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := pg.DeleteTodoListItems(ctx, "work", []int32{5}); err == nil {
		t.Fatalf("expected out-of-range delete error")
	}

	// Unknown list / out-of-range rename are silent no-ops (return nil).
	if err := pg.UpdateTodoListItemData(ctx, "ghost", 0, "x"); err != nil {
		t.Fatalf("rename unknown list should be no-op, got %v", err)
	}
	if err := pg.UpdateTodoListItemData(ctx, "work", 99, "x"); err != nil {
		t.Fatalf("rename out-of-range should be no-op, got %v", err)
	}

	got, err := pg.ShowTodoListItems(ctx, "work")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !reflect.DeepEqual([]string{got[0].Title, got[1].Title}, []string{"buy milk", "call bob"}) {
		t.Fatalf("no-op should not have changed items, got %v", got)
	}
}

// ensureUserTestID is a distinct UUID for a user that is NOT seeded by the
// migration, so it only exists if EnsureUser provisions it.
const ensureUserTestID = "00000000-0000-0000-0000-000000000003"

// TestPgStoreEnsureUser proves just-in-time provisioning: without it, a write by
// a non-placeholder authenticated user violates the lists.user_id FK; after
// EnsureUser, the same write succeeds. It also checks idempotence (a second
// EnsureUser is a no-op, not an error).
func TestPgStoreEnsureUser(t *testing.T) {
	pg := newTestPgStore(t) // skips if no test DB
	ctx := context.Background()

	// Remove any leftover row from a previous run so the FK-violation path is
	// genuinely exercised (TRUNCATE in newTestPgStore does not touch users).
	if _, err := pg.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, ensureUserTestID); err != nil {
		t.Fatalf("cleanup user: %v", err)
	}

	userCtx := WithUserID(ctx, ensureUserTestID)

	// Without provisioning, creating a list must fail on the FK to users(id).
	if err := pg.CreateTodoList(userCtx, "before", nil); err == nil {
		t.Fatal("CreateTodoList for unprovisioned user succeeded, want FK violation")
	}

	// Provision the user, then the same write must succeed.
	if err := pg.EnsureUser(ctx, ensureUserTestID, "ensure@example.com"); err != nil {
		t.Fatalf("EnsureUser: %v", err)
	}
	if err := pg.CreateTodoList(userCtx, "after", []*todo.Item{{Title: "x"}}); err != nil {
		t.Fatalf("CreateTodoList after provisioning: %v", err)
	}

	// Idempotent: provisioning the same user again is a no-op.
	if err := pg.EnsureUser(ctx, ensureUserTestID, "ensure@example.com"); err != nil {
		t.Fatalf("EnsureUser second call (idempotent) returned error: %v", err)
	}

	// Defensive: empty email/user are rejected (provisioning needs a real email).
	if err := pg.EnsureUser(ctx, ensureUserTestID, ""); err == nil {
		t.Fatal("EnsureUser with empty email succeeded, want error")
	}
	if err := pg.EnsureUser(ctx, "", "x@example.com"); err == nil {
		t.Fatal("EnsureUser with empty user id succeeded, want error")
	}
}
