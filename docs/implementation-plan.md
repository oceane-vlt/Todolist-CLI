# Implementation plan — TodoList-CLI toward remote storage

> **Planning document.** No production implementation: this document breaks down the move from the current architecture to the target architecture into incremental, deliverable, testable phases. The excerpts (signatures, commands, proto/SQL snippets) are **illustrative**.
>
> - **Status**: Proposed.
> - **Date**: 2026-06-20.
> - **Builds on**: [`docs/remote-storage-analysis.md`](./remote-storage-analysis.md) (analysis, recommendation §7) and [`docs/target-architecture.md`](./target-architecture.md) (architecture decisions).
> - **Target**: managed PostgreSQL (Neon) + JWT auth (Supabase Auth) + gRPC/TLS + Fly.io hosting + gRPC-Gateway (future web).

## 1. Guiding principle

- **Incremental, never big-bang.** Each phase is independently **deliverable and testable** and **does not break** the project between two phases: at any point, `go test ./...` passes and the CLI stays usable.
- **Storage feature flag**: a `TODO_STORAGE=json|postgres` variable allows switching between the old JSON storage and the new Postgres one throughout the transition. As long as the remote implementation is not validated, we stay on `json` by default. Rollback = set the flag back to `json`.
- **Local de-risking first**: auth is tested with a locally-signed JWT **before** wiring Supabase; TLS is tested with a self-signed certificate **before** deploying to Fly.io.
- **Non-destructive migration**: `~/.config/todolist/data.json` is never deleted during the transition (renamed to `.bak` at most).
- **Documentation consistency**: the plan respects the analysis recommendation (§7) and the ADR decisions (locked choices §1, schema §2, identity via metadata §4, auth flow §5, deployment §6).

## 2. Overview of phases

| # | Phase | Main layers | Effort | Depends on |
| --- | --- | --- | --- | --- |
| 0 | `Store` abstraction (interface) | `libs/storage`, `cmd/server` | **S** | — |
| 1 | Postgres backend behind the interface (single-user, no auth/TLS) | `libs/storage`, `go.mod`, SQL migrations, `Makefile` | **M** | 0 |
| 2 | Multi-user & `user_id` isolation | `libs/storage`, `cmd/server` | **M** | 1 |
| 3 | JWT auth + server interceptor + CLI subcommands | `cmd/server`, `cmd/todo`, `go.mod`, `proto` (optional) | **M/L** | 2 |
| 4 | TLS on the transport | `cmd/server`, `cmd/todo` | **M** | 3 |
| 5 | Migration `data.json` → Postgres + backward compatibility | `cmd/todo`, `libs/storage` | **M** | 1, 2, 3 |
| 6 | Fly.io hosting + secrets + CI | deployment, `Makefile`, `.github/workflows`, docs | **M/L** | 1–5 |
| 7 | gRPC-Gateway + web frontend (**OUT OF v1**) | `proto`, deployment | **L** | 6 |

**Recommended order**: 0 → 1 → 2 → 3 → 4 → 5 → 6, then 7 later (out of the first version). Phases 4 (TLS) and 5 (migration) can be prepared in parallel with phase 3 once phase 2 is done, but the linear order above minimizes risk.

## 3. Detailed phases

### Phase 0 — `Store` abstraction (interface)

- **Goal**: introduce a `Store` interface in `libs/storage` that captures the current persistence behavior, and route `cmd/server` through that interface. **No functional change**: the existing JSON implementation becomes `JSONStore`, which satisfies the interface. This is the **seam** that de-risks everything else.
- **Files / layers touched**: `libs/storage/` (new `store.go` with the interface + adapting the per-operation files `AddData.go`, `createList.go`, `deleteData.go`, `deleteItemsData.go`, `readData.go`, `showData.go`, `UpdateItemData.go`, `common.go`), `cmd/server/main.go` (depends on the interface, not the concrete impl).
- **Dependencies**: none.
- **Validation / tests**: `go test ./...` stays green (the existing tests `AddData_test.go`, `deleteItemsData_test.go`, `readData_test.go` keep passing); CLI behavior **identical** to today (manual smoke test: create/list/show/delete).
- **Risks & de-risking**: low risk. Risk = breaking the signature of existing operations → keep the JSON impl unchanged behind the interface, purely mechanical refactoring.
- **Effort**: **S**.

> Illustrative interface:
> ```go
> type Store interface {
>     GetLists(ctx context.Context) ([]ListSummary, error)
>     CreateList(ctx context.Context, title string, items []Item) error
>     // ... one method per existing business RPC
> }
> ```

### Phase 1 — Postgres backend behind the interface (single-user, no auth/TLS)

- **Goal**: add a `PgStore` implementation (via `pgx`) that satisfies the `Store` interface, with the SQL schema from ADR §2.2 — but with a **fixed/placeholder** `user_id` for this phase (multi-user comes in phase 2). Wire the **feature flag** `TODO_STORAGE=json|postgres`.
- **Files / layers touched**: `libs/storage/pgstore.go` (new), SQL migrations (`migrations/0001_init.sql`), impl selection based on `TODO_STORAGE` (in `cmd/server/main.go` or a `libs/storage` factory), `go.mod` (add `pgx`), `Makefile` (migration target + possibly a local Postgres `docker-compose` for tests).
- **Dependencies**: Phase 0 (the interface must exist).
- **Validation / tests**: `PgStore` integration tests against a local Postgres (Docker); **parity tests** comparing the result of `JSONStore` vs `PgStore` operations on the same inputs; `TODO_STORAGE=postgres` lets the CLI run against Postgres in single-user mode.
- **Risks & de-risking**: schema/model divergence between JSON and SQL → systematic **parity tests**; migration tooling to decide (`golang-migrate` vs `sqlc` vs raw SQL) → choose before coding the phase (see §5).
- **Effort**: **M**.

### Phase 2 — Multi-user & `user_id` isolation

- **Goal**: propagate a `user_id` (read from the `context.Context`) into `PgStore` (`WHERE user_id = $1`, logical key `(user_id, title)`) and into the `cmd/server` handlers (scoping). The `user_id` is still **hard-injected / via a dev variable** as long as auth (phase 3) is not wired.
- **Files / layers touched**: `libs/storage/pgstore.go` (signatures take `userID`, filtered queries), `cmd/server/main.go` (handlers pass the context `user_id` to the store).
- **Dependencies**: Phase 1.
- **Validation / tests**: isolation test — two distinct `user_id`s do **not** see each other's lists; `UNIQUE(user_id, title)` allows the same title for two different users; the old tests stay green.
- **Risks & de-risking**: a forgotten `user_id` filter in a query (data leak) → targeted review + isolation test covering **every** RPC.
- **Effort**: **M**.

### Phase 3 — JWT auth + server interceptor + CLI subcommands

- **Goal**: actually authenticate the caller. Server side: a **gRPC interceptor** validates the JWT (signature + expiration) and injects the `user_id` into the `context` (which feeds phase 2). CLI side: `login` / `signup` / `logout` subcommands, storing the token in `~/.config/todolist/credentials.json` (perms `0600`), attaching `authorization: Bearer <access JWT>` as metadata, transparent refresh on `Unauthenticated`.
- **Files / layers touched**: `cmd/server/main.go` (unary interceptor), `cmd/todo/` (`login`/`signup`/`logout` commands, `credentials.json` handling, token attachment, refresh logic), `go.mod` (add `golang-jwt`), `proto/todoList.proto` (**optional**: `AuthService` Signup/Login/Refresh **only in home-grown auth mode**; not needed with Supabase Auth, cf. ADR §4.2).
- **Dependencies**: Phase 2 (the context `user_id` must already scope storage).
- **Validation / tests**: call **without** a token → `Unauthenticated`; **expired** token → `Unauthenticated` then transparent refresh replays the call; valid token → access only to the user's own lists; `credentials.json` created with `0600`.
- **Risks & de-risking**: Supabase integration complexity → **de-risk locally first** with a JWT signed by a test key (home-grown mode), validate the whole interceptor/refresh flow, **then** wire Supabase JWT validation (JWKS/secret). Signing secret to protect (env, never in the repo).
- **Effort**: **M/L**.

### Phase 4 — TLS on the transport

- **Goal**: encrypt the gRPC transport (end of `insecure.NewCredentials()`). Server side: TLS credentials. CLI side: TLS credentials, **configurable endpoint** (`TODO_SERVER_ENDPOINT` / `--endpoint`, replaces the hard-coded `127.0.0.1:50051`), and adding **timeouts/retries** (remote network).
- **Files / layers touched**: `cmd/server/main.go` (`credentials.NewTLS`), `cmd/todo/main.go` (TLS creds, configurable endpoint, `context.WithTimeout`, retry policy).
- **Dependencies**: Phase 3 (tokens now travel over a channel that must be encrypted).
- **Validation / tests**: **insecure connection refused** by the TLS server; successful TLS handshake with a local self-signed certificate; a call exceeding the timeout fails cleanly (no hang).
- **Risks & de-risking**: TLS misconfiguration → **test with a local self-signed certificate** before any remote deployment (phase 6). On Fly.io, TLS may be terminated by the PaaS (cf. ADR §6.2).
- **Effort**: **M**.

### Phase 5 — Migration `data.json` → Postgres + backward compatibility

- **Goal**: let an existing user transfer their `~/.config/todolist/data.json` to Postgres for their logged-in account, without loss. One-shot `todo migrate` command: reads the local JSON, **upserts** into Postgres for the current `user_id`, **idempotent** (replayable without duplicates thanks to `UNIQUE(user_id, title)`). The `data.json` is **kept** (renamed `.bak`), never deleted.
- **Files / layers touched**: `cmd/todo/` (`migrate` subcommand), `libs/storage` (reuse existing JSON reading + write via `PgStore`).
- **Dependencies**: Phases 1 (PgStore), 2 (`user_id` scoping), 3 (the user must be authenticated to have a `user_id`).
- **Validation / tests**: migrating a sample `data.json` → lists/items present in the database under the correct `user_id`; **idempotent re-run** (no duplicate, no error); the original file is preserved as `.bak`.
- **Risks & de-risking**: data loss → never delete `data.json`, idempotent migration, dry-run possible. Backward compat ensured by the **feature flag**: as long as `TODO_STORAGE=json`, nothing changes for the user; switch to `postgres` after the migration is validated.
- **Effort**: **M**.

### Phase 6 — Fly.io hosting + secrets + CI

- **Goal**: deploy the gRPC server on a free-tier PaaS (Fly.io), publicly accessible over TLS, and **replace the local launchd**. Set up secrets and a CI.
- **Files / layers touched**: `Dockerfile` (server), `fly.toml`, secrets via `fly secrets` (`DATABASE_URL`, `SUPABASE_JWT_SECRET`/`SUPABASE_JWKS_URL`, `TLS_*`, `PORT` — cf. ADR §6.2), documentation (`docs/daemon-setup.md` → remote deployment guide; the CLI no longer launches a server), `.github/workflows/` (**new** CI: build + `go test` + `golangci-lint`, absent today).
- **Dependencies**: Phases 1–5 (the server must be complete and secure before being publicly exposed).
- **Validation / tests**: deployment to a Fly.io staging environment; **smoke test** of the remote CLI (`login` then `create`/`list` against the deployed endpoint); CI passes on a PR.
- **Risks & de-risking**: secret committed by mistake → secrets exclusively via the Fly.io secret manager, `.gitignore` checked, secret scan in CI. Free-tier sleep → document the behavior (cold start). Deploy to **staging** before prod.
- **Effort**: **M/L**.

### Phase 7 — gRPC-Gateway + web frontend (OUT OF v1)

- **Goal**: expose the same gRPC service as REST/JSON via **gRPC-Gateway** (`google.api.http` annotations on the RPCs), for a future web frontend (cf. ADR §3 and analysis §6).
- **Files / layers touched**: `proto/todoList.proto` (HTTP annotations), Gateway generation (`Makefile`/`buf`), deployment (expose the Gateway).
- **Dependencies**: Phase 6.
- **Status**: **explicitly out of the first version** — to be planned after the v1 remote CLI is stabilized.
- **Effort**: **L**.

## 4. Migration and backward-compatibility management

- **Feature flag `TODO_STORAGE`** (`json` | `postgres`) active **throughout the transition**: defaults to `json` until the remote path is fully validated, then switches to `postgres`. Allows instant rollback.
- **`data.json` never destroyed**: the migration (phase 5) renames it to `.bak` at most; no automatic deletion.
- **Idempotent `todo migrate` tool**: replayable without creating duplicates (guaranteed by `UNIQUE(user_id, title)`), so safe in case of interruption.
- **Rollback**: setting `TODO_STORAGE=json` back immediately restores the original local behavior (the local data is still there).
- **No breakage window**: between two phases, the project compiles, the tests pass and the CLI works (on JSON by default).

## 5. New Go dependencies, config, secrets and CI

### Go dependencies to add

| Dependency | Role | Phase |
| --- | --- | --- |
| `github.com/jackc/pgx` (v5) | Postgres driver / pool, parameterized queries | 1 |
| `golang-migrate` **or** `sqlc` (to decide) | SQL migrations / typed code generation from SQL | 1 |
| `github.com/golang-jwt/jwt` (v5) | JWT validation/parsing on the server side | 3 |
| `supabase-go` (possible) | CLI ↔ Supabase Auth dialogue if we don't hit the HTTP API directly | 3 |

> **Decision to make at the start of phase 1**: `golang-migrate` (versioned migrations, raw SQL) vs `sqlc` (generates typed Go from SQL queries). Recommendation: decide before writing `PgStore` so as not to mix approaches.

### Config / secrets (cf. ADR §6.2)

- **Server**: `DATABASE_URL`, `SUPABASE_JWT_SECRET` / `SUPABASE_JWKS_URL`, `JWT_SIGNING_KEY` (home-grown mode), `TLS_CERT_PATH` / `TLS_KEY_PATH`, `PORT` — via the Fly.io secret manager, **never** in the repo.
- **CLI**: `TODO_SERVER_ENDPOINT` (or `--endpoint`), `TODO_STORAGE` (transition), `~/.config/todolist/credentials.json` (0600).

### CI (new)

- No `.github/workflows` today. Add (phase 6) a GitHub Actions pipeline: `go build`, `go test ./...`, `golangci-lint` (the `lint` target in the `Makefile` already exists), and ideally a Postgres integration job (service container).

## 6. Cross-cutting risks and de-risking strategy

| Risk | Phase(s) | De-risking |
| --- | --- | --- |
| JSON ↔ Postgres behavior divergence | 1 | `JSONStore` vs `PgStore` parity tests |
| Data leak (forgotten `user_id` filter) | 2 | Per-RPC isolation test + targeted review |
| Complex / fragile auth integration | 3 | Locally-signed JWT first, **then** Supabase; refresh tested offline |
| Bad TLS config | 4, 6 | Self-signed certificate locally before Fly.io |
| Data loss during migration | 5 | `data.json` kept as `.bak`, idempotent migration, feature flag |
| Secret committed / exposed | 6 | PaaS secret manager, secret scan in CI, `.gitignore` |
| Breakage between two phases | all | `TODO_STORAGE` feature flag, every phase testable, tests green continuously |

## 7. Out of scope for the first version

- **Web frontend** and **gRPC-Gateway** (phase 7) — come after the v1 remote CLI.
- **Real-time / notifications** (push sync between devices) — v1 consistency relies on the database, not on push.
- **List sharing between users** — the v1 model strictly isolates by `user_id`.
- **Additional OAuth providers** (GitHub/Google) if Supabase Auth already covers email/password for v1.
- **Advanced observability** (distributed tracing, dashboards) — beyond basic server logs.

---

*Planning document only — no production implementation. Excerpts (signatures, SQL, proto) provided for illustration. Consistent with `docs/remote-storage-analysis.md` and `docs/target-architecture.md`.*
