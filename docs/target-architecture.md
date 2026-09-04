# Target architecture — TodoList-CLI with remote storage

> **Architecture Decision Record (ADR).** Not a production implementation: this document settles the technical choices and describes the target. The SQL DDL examples and signatures are **illustrative**.
>
> - **Status**: Decided (builds on [`docs/remote-storage-analysis.md`](./remote-storage-analysis.md), recommendation §7).
> - **Date**: 2026-06-20.
> - **Next step** (out of scope): detailed implementation plan, then code.

## 1. Settled solution choices

Directly carried over from the analysis recommendation (see `remote-storage-analysis.md` §7), with a short justification.

| Domain | Settled choice | Justification (analysis reference) |
| --- | --- | --- |
| **Storage backend** | **Managed PostgreSQL on a free tier — Neon** (Supabase = fallback) | Analysis §3 Option A & §4: native multi-device/multi-user via SQL transactions + constraints, free/durable, relational model aligned with the README's "SQLite planned". |
| **Auth** | **JWTs** validated server-side, issued by **Supabase Auth** (fallback: homegrown JWT + GitHub/Google OAuth) | Analysis §5 & §7: standard, stateless on the server, portable to the web; Supabase Auth reduces auth code. |
| **Transport / TLS** | **gRPC over TLS** (end of `insecure.NewCredentials()`) | Analysis §2 gap #2 & §8: mandatory as soon as we leave localhost, otherwise tokens travel in clear text. |
| **Server hosting** | **PaaS free tier — Fly.io** (fallbacks: Railway, Render) | Analysis §3 Option A: hosts the gRPC server; avoids the ops/security burden of a VPS (Option E rejected). |
| **Future web frontend** | **gRPC-Gateway** (REST/JSON generated from the `.proto`); gRPC-Web as a possible complement | Analysis §6 & §7: a single gRPC backend, two clients (CLI + web), business logic/security centralized. |
| **Data isolation** | **By `user_id`**, enforced in every RPC and every SQL query | Analysis §2 gap #3 & §8: never trust the client for the data scope. |

**Guiding principle**: the gRPC server remains the **sole guardian of the data** (the only one that talks to the database). We change the persistence layer, not the gRPC seam. This is what distinguishes the chosen option (A) from Firestore (B, rejected because it bypasses the backend).

## 2. Target data schema

### 2.1 From the current JSON model to the relational model

Current model (`libs/storage/common.go`): a single JSON file, single-user, lists indexed by title.

```go
// Current — a single file, no notion of user
type TodoData struct {
    Lists map[string][]TodoItem // key = title
}
```

| Aspect | Current JSON model | Target SQL model |
| --- | --- | --- |
| Scope | Single-user (local file) | Multi-user (`user_id` everywhere) |
| Key of a list | `title` (global) | `(user_id, title)` unique |
| Items | array in `map[title]` | `items` table (FK to `lists`), explicit ordering |
| Concurrency | full file rewrite, no lock | transactions + ACID constraints |
| Cascade delete | manual (map rewrite) | `ON DELETE CASCADE` |
| `priority` | free-form string | enum / `CHECK` constraint |

### 2.2 Tables, keys and constraints

Three tables: `users` (can be delegated to Supabase Auth), `lists`, `items`.

- **`users`** — identity. PK `id`. If Supabase Auth is used, this table = `auth.users` managed by Supabase; we simply reference its `id` (UUID).
- **`lists`** — a todolist owned by a user. **Key constraint**: `UNIQUE(user_id, title)` (replaces indexing by title alone). FK `user_id → users(id) ON DELETE CASCADE`.
- **`items`** — the elements of a list. FK `list_id → lists(id) ON DELETE CASCADE`. `position` to preserve order (the JSON relied on the array order and on `item_index`).

**Illustrative** DDL (Postgres target):

```sql
-- users: managed by Supabase Auth (auth.users) if Supabase; otherwise a homegrown table.
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT UNIQUE NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE lists (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, title)            -- isolation + uniqueness per user
);

CREATE TABLE items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    list_id     UUID NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    position    INT  NOT NULL,         -- stable ordering (replaces item_index)
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    completed   BOOLEAN NOT NULL DEFAULT false,
    due_date    DATE,
    priority    TEXT NOT NULL DEFAULT 'none'
                CHECK (priority IN ('none','low','medium','high')),
    UNIQUE (list_id, position)
);

CREATE INDEX idx_lists_user  ON lists(user_id);
CREATE INDEX idx_items_list  ON items(list_id);
```

> Field mapping: `Item{title, description, completed, dueDate, priority}` from the `.proto` → `items` columns. `dueDate` (free-form string on the proto side) → `DATE` (or `TEXT` if we want to preserve the raw format at first). `priority` (free-form string) → `CHECK` constraint.

## 3. End-to-end target architecture

The CLI never talks to the database; it always goes through the remote gRPC server, over TLS, with a JWT in the metadata. The future web frontend uses the **same** server via gRPC-Gateway (REST/JSON) or gRPC-Web.

```
                         TODAY (local, single-user)
   ┌──────────────┐   gRPC insecure    ┌───────────────┐   os.WriteFile   ┌──────────────────┐
   │  cmd/todo    │ ─────────────────▶ │  cmd/server   │ ───────────────▶ │ ~/.config/.../   │
   │  (CLI)       │   127.0.0.1:50051  │  (gRPC local) │   (JSON 0644)    │   data.json      │
   └──────────────┘                    └───────────────┘                  └──────────────────┘
                                   (launchd daemon macOS)


                          TARGET (remote, multi-user, multi-device)

   ┌──────────────┐                                          ┌───────────────────────────┐
   │  cmd/todo    │                                          │  Supabase Auth (IdP)      │
   │  (CLI)       │ ── signup/login (HTTPS) ───────────────▶ │  issues access + refresh JWT │
   │              │ ◀── access JWT + refresh JWT ──────────  └───────────────────────────┘
   │  token store │
   │  0600        │
   └──────┬───────┘
          │  gRPC over TLS
          │  metadata: authorization: Bearer <access JWT>
          ▼
   ┌─────────────────────────────────────────┐         ┌──────────────────────────────┐
   │      cmd/server  (gRPC, hosted on PaaS)   │  pgx /  │   Managed PostgreSQL (Neon)  │
   │  ┌─────────────────────────────────────┐ │  SQL    │   users/lists/items tables   │
   │  │ TLS + auth interceptor:             │ │ ──────▶ │   isolation by user_id       │
   │  │  validates JWT → user_id in context │ │ ◀────── │   transactions / constraints │
   │  └─────────────────────────────────────┘ │         └──────────────────────────────┘
   │  business RPCs: scoping by user_id        │
   └──────────────▲────────────────────────────┘
                  │ (future)
   ┌──────────────┴───────────────┐
   │  gRPC-Gateway (REST/JSON)     │ ◀── Future web frontend (HTTPS / gRPC-Web)
   │  generated from the .proto    │
   └──────────────────────────────┘
```

Flow of a business call (e.g. `GetTodoLists`):

1. The CLI reads the access JWT from its local store and attaches it to the gRPC metadata (`authorization: Bearer …`).
2. **TLS** connection to the remote gRPC server.
3. The server-side **interceptor** validates the JWT (signature + expiry), extracts the `user_id`, and injects it into the `context.Context`.
4. The RPC handler reads the `user_id` from the context and runs a **parameterized** SQL query filtered by `WHERE user_id = $1`.
5. The response is returned to the CLI (or to the web frontend via the Gateway).

## 4. Impact on the `proto` contract and on the code

### 4.1 Where identity flows: metadata, NOT a message field

- **Decision**: the user's identity travels through the **gRPC metadata** (`authorization: Bearer <JWT>`), **never** through a `user_id` field in the messages — otherwise a malicious client could impersonate another user (analysis §5 and §6).
- The `user_id` is derived from the JWT **on the server side** by the interceptor, then read from the `context`. The client neither provides it nor sees it.

### 4.2 `proto` contract — what changes and what does not

- The **7 business RPCs keep their names and their shape** (`CreateTodoList`, `GetTodoLists`, `ShowTodoListItems`, `DeleteTodoList`, `DeleteTodoListItems`, `UpdateTodoList`, `UpdateTodoListItem`). They continue to index by `title` on the client side; the server enforces the logical key **`(user_id, title)`**.
- **Amendment (item descriptions).** `UpdateTodoListItemRequest` has since gained
  `new_description`, and its `new_title` / `new_description` fields are proto3
  **optional**: an absent field leaves the current value untouched, while a field
  set to `""` clears it. This is what makes `PATCH /v1/lists/{title}/items` a
  genuine partial update — editing a description must not wipe the title. The
  change is wire-compatible (same field numbers, presence only adds a synthetic
  oneof), so it does not break the "unchanged contract" guarantee this section
  was written to express.
- **No `user_id` field added** to the messages (see 4.1).
- **Option (future)**: a separate `AuthService` if we do NOT use managed auth:

  ```proto
  // Illustrative — only if homegrown auth (otherwise Supabase Auth handles it)
  service AuthService {
      rpc Signup  (SignupRequest)  returns (TokenResponse);
      rpc Login   (LoginRequest)   returns (TokenResponse);
      rpc Refresh (RefreshRequest) returns (TokenResponse);
  }
  message TokenResponse {
      string access_token  = 1;
      string refresh_token = 2;
      int64  expires_in    = 3;
  }
  ```

  With **Supabase Auth**, this service is not needed: the CLI talks directly to the Supabase auth endpoint (HTTPS), and the gRPC server merely **validates** the JWTs issued by Supabase.

- **Future web**: `google.api.http` annotations on the RPCs to generate the REST layer via **gRPC-Gateway**.

### 4.3 Affected code layers

| Layer | Today | Target |
| --- | --- | --- |
| `libs/storage` | JSON `os.WriteFile` (full rewrite) | **Postgres access via `pgx`**: parameterized queries scoping by `user_id`, transactions. Signatures take a `user_id` (from the context). This is the **bulk of the change**. |
| `cmd/server` | local gRPC, no interceptor, no TLS | Add **TLS** (creds), an **auth interceptor** (JWT → `user_id` in context), config read from env, **`user_id` scoping** in each handler. |
| `cmd/todo` (CLI) | `insecure` dial to 127.0.0.1, no token, no timeout | **TLS creds**, **configurable** endpoint (env/flag/config), **attaches the JWT** in metadata, **timeouts/retries** (remote network), new subcommands **`login` / `signup` / `logout`**, **token storage/refresh**. |
| `proto` | 7 RPCs, no notion of user | **7 RPCs unchanged**; (optional) `AuthService`; (future) HTTP annotations for the Gateway. |

Illustrative signature of the storage layer (target):

```go
// Illustrative — user_id comes from the context (injected by the interceptor), not from a client argument
func (s *Store) GetLists(ctx context.Context, userID string) ([]ListSummary, error)
func (s *Store) CreateList(ctx context.Context, userID, title string, items []Item) error
```

## 5. Authentication flow and multi-device

### 5.1 Signup / Login

```
1. todo signup --email <e> (password entered via prompt)     ┐
   todo login  --email <e>                                    │ HTTPS → Supabase Auth
2. Supabase Auth verifies and returns { access_token (short), ┘
   refresh_token (long) }
3. The CLI writes the tokens to ~/.config/todolist/credentials.json  (perms 0600)
4. Business gRPC calls attach: authorization: Bearer <access_token>
```

### 5.2 Client-side token storage

- File **`~/.config/todolist/credentials.json`**, permissions **`0600`** (owner read/write only). This is the same directory as the current `data.json`, but the latter goes away (data now lives in the database).
- Contents: `access_token`, `refresh_token`, `expires_at`, `endpoint` (server). **No secret in the repo.**

### 5.3 Refresh

- When the access JWT is expired (an `Unauthenticated` response or a past `expires_at`), the CLI uses the **refresh_token** to obtain a new access JWT (from Supabase Auth, or via `AuthService.Refresh` in homegrown mode), then replays the call. Transparent to the user.
- `logout` deletes `credentials.json` (and revokes the refresh token on the provider side if supported).

### 5.4 Multi-device

```
PC A ──login──▶ Supabase Auth ──▶ credentials.json (PC A)  ┐
PC B ──login──▶ Supabase Auth ──▶ credentials.json (PC B)  ┘  same account (same user_id)
        │                                  │
        └────── gRPC over TLS ─────────────┘
                        ▼
              remote gRPC server
                        ▼
          Postgres (lists/items WHERE user_id = …)
```

- Same account signed in on several machines; **each device has its own tokens** but the **same `user_id`** → it sees the same data.
- **Consistency** across devices is guaranteed by the database (transactions, `UNIQUE(user_id, title)`), which resolves gap #4 from the analysis (concurrent writes) that the JSON file did not handle.

### 5.5 Server-side JWT verification (auth modes)

The server only **validates** incoming JWTs (signature + expiry + `sub`); it
never talks to Supabase. Three modes exist, selected by environment in
`AuthInterceptorFromEnv` (`server/authconfig.go`), from most specific to the
fallback:

| Precedence | Variable(s) | Verifier | Usage |
| --- | --- | --- | --- |
| 1 (always wins) | `JWT_SIGNING_KEY` | `HMACVerifier` (HS256) | **home/dev** override — a developer can always force the homegrown secret. |
| 2 | `SUPABASE_URL` **or** `SUPABASE_JWKS_URL` | `JWKSVerifier` (ES256/RS256) | **JWKS / target mode (Option B)**: validates asymmetric tokens via the project's public key. |
| 3 | `SUPABASE_JWT_SECRET` | `HMACVerifier` (HS256) | **Legacy (Option A)**: validates HS256 tokens against the project's shared secret. |
| 4 (default) | — (none) | `DevUserIDInterceptor` | Auth OFF — the default local run keeps working. |

- **JWKS mode (Option B, target).** When `SUPABASE_URL` is set, the server
  derives the JWKS endpoint `{SUPABASE_URL}/auth/v1/.well-known/jwks.json` (same
  `/auth/v1` prefix as the CLI). `SUPABASE_JWKS_URL` lets you **override** that
  endpoint with a full URL (tests / non-standard deployments) and takes priority
  over `SUPABASE_URL`. The `JWKSVerifier` fetches the public key(s) **lazily on
  the first `Verify`** (server startup does not depend on the JWKS being
  reachable), caches them by `kid` with a TTL and a refetch on an unknown `kid`
  (key rotation), and verifies the signature in ES256/RS256.
- **The Supabase project's ACTIVE signing key can stay ECC/ES256.** That is
  precisely the point of Option B: no longer any need to downgrade the project to
  HS256. The server validates the asymmetric tokens directly via the public JWKS.
- **Forcing legacy HS256 (Option A).** To stay on the shared HS256 secret, set
  only `SUPABASE_JWT_SECRET` on the server side (without `SUPABASE_URL` or
  `SUPABASE_JWKS_URL`). This assumes the project still issues HS256 tokens.
- All three modes return the same `Identity{UserID: sub, Email: <email claim>}`,
  so the `user_id` isolation and the JIT provisioning of `users` work identically
  regardless of the mode.

## 6. Configuration and deployment changes

### 6.1 From the local launchd daemon to a hosted server

- **Today**: the server runs locally via **launchd** (`docs/daemon-setup.md`) on the user's machine, listening on `127.0.0.1:50051`.
- **Target**: the server is **hosted on a free-tier PaaS (Fly.io)**, publicly reachable over **TLS**. **End of the local launchd**; `daemon-setup.md` will become "remote server deployment". The local CLI no longer starts a server.

### 6.2 Environment variables and secrets

On the **server** side (never in the repo — PaaS secrets / env):

| Variable | Role |
| --- | --- |
| `DATABASE_URL` | Postgres (Neon) connection string — secret |
| `SUPABASE_URL` | Bare project URL (`https://<ref>.supabase.co`) → enables **JWKS / ES256 mode (Option B)**; the server derives `{SUPABASE_URL}/auth/v1/.well-known/jwks.json` |
| `SUPABASE_JWKS_URL` | Override of the JWKS endpoint (full URL); takes priority over `SUPABASE_URL` |
| `SUPABASE_JWT_SECRET` | Legacy HS256 secret to **validate** the JWTs (Supabase **Option A** mode) |
| `JWT_SIGNING_KEY` | Signing secret (homegrown/dev auth mode — wins over everything else) |
| `TLS_CERT_PATH` / `TLS_KEY_PATH` | TLS certificate (or TLS terminated by the PaaS) |
| `PORT` | Listening port provided by the PaaS |

Server auth precedence: `JWT_SIGNING_KEY` (home/dev) > `SUPABASE_URL`/`SUPABASE_JWKS_URL` (JWKS, target) > `SUPABASE_JWT_SECRET` (HS256 legacy) > none (auth OFF). Details in §5.5.

On the **CLI** side:

| Variable / flag | Role |
| --- | --- |
| `TODO_SERVER_ENDPOINT` (env) or `--endpoint` (flag) or config | Address of the remote gRPC server (replaces the hard-coded `127.0.0.1:50051`) |
| `~/.config/todolist/credentials.json` (0600) | Auth tokens (generated by `login`) |

### 6.3 Secret management

- Secrets stored via the **PaaS secret manager** (Fly.io secrets) and/or runtime env variables. **Never** committed.
- `DATABASE_URL` and the JWT secret never leave the server; the CLI knows **only** its endpoint and its tokens.

## 7. Security prerequisites (reminder from analysis §8)

Carried over from `remote-storage-analysis.md` §8 — to be addressed at implementation time (next step):

- [ ] **TLS** on the gRPC transport (replaces `insecure.NewCredentials()`).
- [ ] **Server-side authentication interceptor**: JWT validation → `user_id` in the `context`.
- [ ] **Isolation by `user_id`** in every RPC and every **parameterized** SQL query.
- [ ] **Secrets** (`DATABASE_URL`, JWT secret) via env / secret manager, outside the repo.
- [ ] **Timeouts / retries** on the gRPC client side (absent today), for a remote network.
- [ ] **Backups** and a database restore plan.

---

*Decision/design document only — no production implementation. DDL and signatures provided for illustration. Next step: detailed implementation plan.*
