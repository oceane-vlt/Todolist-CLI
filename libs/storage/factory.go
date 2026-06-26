package storage

import (
	"context"
	"fmt"
	"os"
)

// Backend names accepted by the TODO_STORAGE feature flag.
const (
	BackendJSON     = "json"
	BackendPostgres = "postgres"
)

// EnvStorageBackend is the environment variable that selects the storage
// backend at runtime (Phase 1 feature flag, docs/implementation-plan.md §1).
// Empty or unset defaults to the local JSON backend so existing behaviour is
// preserved with no configuration.
const EnvStorageBackend = "TODO_STORAGE"

// EnvDatabaseURL is the Postgres connection string used when TODO_STORAGE=postgres.
const EnvDatabaseURL = "DATABASE_URL"

// NewStoreFromEnv builds a Store according to the TODO_STORAGE feature flag:
//
//   - unset or "json": local single-file JSON store at jsonPath (current default).
//   - "postgres":       pgx-backed PgStore using DATABASE_URL.
//
// The caller is responsible for closing the returned Store when it is a PgStore
// (use the returned closeFn, which is a no-op for the JSON backend). Keeping the
// selection here means cmd/server stays a thin wiring layer.
func NewStoreFromEnv(ctx context.Context, jsonPath string) (store Store, closeFn func(), err error) {
	backend := os.Getenv(EnvStorageBackend)
	switch backend {
	case "", BackendJSON:
		return NewJSONStore(jsonPath), func() {}, nil
	case BackendPostgres:
		connString := os.Getenv(EnvDatabaseURL)
		if connString == "" {
			return nil, nil, fmt.Errorf("%s=%s requires %s to be set", EnvStorageBackend, BackendPostgres, EnvDatabaseURL)
		}
		pg, err := NewPgStorePool(ctx, connString)
		if err != nil {
			return nil, nil, err
		}
		return pg, pg.Close, nil
	default:
		return nil, nil, fmt.Errorf("unknown %s value %q (expected %q or %q)", EnvStorageBackend, backend, BackendJSON, BackendPostgres)
	}
}
