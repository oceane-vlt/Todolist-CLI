# Analysis: moving from local storage to remote storage

> **Analysis** document (not implementation). Goal: be able to access your todolists from multiple machines, while keeping a backend reusable by a future web frontend.

## 1. Context and current architecture

State of the code (verified against the sources):

| Layer | File | Current state |
| --- | --- | --- |
| CLI client | `cmd/todo/main.go` | gRPC client, `dial 127.0.0.1:50051`, `insecure.NewCredentials()` — **no TLS, no auth, no timeout/retry** |
| Server | `cmd/server/main.go` | gRPC server, `net.Listen("tcp","127.0.0.1:50051")` — **single-user, no auth, no TLS, no interceptors**; launchd daemon (macOS) |
| Contract | `proto/todoList.proto` | `TodoListService`, 7 RPCs. Lists indexed by **Title** only. **No notion of account/user** |
| Storage | `libs/storage/common.go` | `TodoData{Lists map[string][]TodoItem}` serialized into a **single JSON file** `~/.config/todolist/data.json` via `os.WriteFile` (full rewrite, `0644`). **No lock, no transaction, no concurrency control** |
| Dependencies | `go.mod` | go 1.24, grpc v1.76, protobuf v1.36, cobra, promptui. **No DB driver, no HTTP framework, no auth lib** |

The layering is clean:

```
cmd/todo (client) -> proto (contract) -> server (gRPC impl) -> libs/storage (persistence)
```

**The gRPC contract is the natural seam** to preserve. We replace `libs/storage` (local JSON) with access to a remote database, behind the same gRPC server. A future web frontend will be able to reuse this backend via **gRPC-Web** or **gRPC-Gateway** (REST/JSON generated from the `.proto`).

## 2. Gaps to close for remote / multi-device / multi-user

1. **Authentication + authorization**: nonexistent today.
2. **Transport security (TLS)**: `insecure` connection today.
3. **Per-user data isolation**: lists are global (key = Title).
4. **Concurrent-write safety**: none (full file rewrite).
5. **Publicly reachable** server hosting.

These points are **cross-cutting**: they apply whatever storage option is chosen.

## 3. Remote storage / backend options

Criteria: cost (free if possible), security, multi-device, multi-user, hosting, and **reusability by a web frontend**.

### Option A — Managed PostgreSQL on a free tier (Supabase / Neon / Fly.io Postgres)

- **Principle**: the current gRPC server remains the only one talking to the database; we replace `libs/storage` with SQL access (`pgx`/`database/sql`). We host the gRPC server somewhere (Fly.io, Railway, Render free tier).
- **Cost**: Neon and Supabase have durable free tiers; Fly.io / Railway / Render too (with limits/idling).
- **Multi-device / multi-user**: excellent. `users` + `lists` + `items` schema with `user_id`; SQL transactions and constraints natively solve concurrency (gap #4).
- **Web reuse**: the web frontend hits the **same gRPC server** (via gRPC-Gateway/gRPC-Web), not the database directly → shared backend, centralized business logic.
- **Security risks**:
  - Connection string (secret) must be protected (env variables / secret manager, never in the repo).
  - Attack surface = the exposed gRPC server: **requires TLS + auth + `user_id` isolation** (otherwise anyone can read all the lists).
  - SQL injection if queries are concatenated → use parameterized queries.
- **Verdict**: aligned with the current architecture (keeps the gRPC server as the sole data gatekeeper), scales well, and **matches the README's "SQLite planned"** (Postgres = same relational model, but remote).

### Option B — Firebase / Firestore (BaaS, NoSQL)

- **Principle**: the clients (CLI **and** web frontend) talk directly to Firestore via the SDKs, with security rules on the Firebase side.
- **Cost**: generous free tier (Spark) for personal use.
- **Multi-device / multi-user**: native, real-time sync included.
- **Web reuse**: very good (first-class web SDK).
- **Security risks**:
  - **Bypasses the gRPC server**: security rests entirely on Firestore *Security Rules* (easy to misconfigure → data leak). No Go admin SDK on the CLI side without a service account to protect.
  - Strong lock-in to the Google ecosystem.
  - The NoSQL data model does not match the current relational model; the schema would need to be rethought.
- **Verdict**: quick to start but **breaks the gRPC seam** (the business backend is no longer central) → less aligned with the "same reused backend" goal.

### Option C — SQLite + Litestream (replication to object storage)

- **Principle**: keep SQLite local on the server side, continuously replicated to object storage (S3/Backblaze B2).
- **Cost**: very low.
- **Multi-device / multi-user**: **poor** for concurrent multi-machine writes. Litestream = replication/restore (DR), **not** a multi-writer database. Does not solve multi-device writes.
- **Verdict**: **discarded** for this goal (multi-device writes are the core need).

### Option D — MongoDB Atlas (free tier M0)

- **Principle**: managed document database, accessed by the gRPC server.
- **Cost**: free tier M0 (512 MB).
- **Multi-device / multi-user**: good.
- **Web reuse**: good via the gRPC server.
- **Security risks**: network config (IP allowlist), connection secret; historically, misconfigured instances have leaked. Document model to be designed.
- **Verdict**: viable, but the relational model (Postgres) fits the structured todolist data and the "SQLite planned" better.

### Option E — Self-hosted VPS (Postgres + gRPC server on the same VM)

- **Principle**: a small VPS (Oracle Cloud Free Tier, etc.), full control.
- **Cost**: possibly free (Oracle Always Free) but variable.
- **Security risks**: **the entire ops/security burden falls on you** (OS patches, firewall, fail2ban, TLS renewal, backups). High surface and effort.
- **Verdict**: flexible but costly in maintenance/security for a personal project.

## 4. Summary comparison

| Option | Free cost | Multi-device (writes) | Multi-user | Reuses the gRPC server | Security/ops effort | Aligned with current arch |
| --- | --- | --- | --- | --- | --- | --- |
| A. Managed Postgres | Yes (Neon/Supabase) | Excellent | Excellent | **Yes** | Medium | **Strong** |
| B. Firestore | Yes | Excellent | Excellent | No (short-circuited) | Medium (rules) | Weak |
| C. SQLite+Litestream | Yes | **Poor** | Poor | Yes | Low | Medium |
| D. MongoDB Atlas | Yes (M0) | Good | Good | Yes | Medium | Medium |
| E. Self-hosted VPS | Variable | Good | Good | Yes | **High** | Medium |

## 5. Authentication — options and risks

Whatever the database, the **gRPC server** must authenticate the caller (except option B where Firebase does it).

| Approach | Description | Pros | Risks / limits |
| --- | --- | --- | --- |
| **Per-user API key** | Opaque token sent in gRPC metadata | Simple to implement | Revocation/rotation to manage; if leaked, full access; no scope standard |
| **Signed (short) JWT + refresh** | The server issues a JWT after login | Standard, stateless on the server side, web-portable | Expiration/refresh handling; signing secret to protect; revocation = needs a blacklist |
| **OAuth/OIDC (Google, GitHub)** | Identity delegation to a provider | No password management; good for the future web frontend | More complex to wire; dependency on an IdP |
| **Managed auth (Supabase Auth / Firebase Auth)** | The BaaS provides login + tokens | Reduces auth code; issues server-verifiable JWTs | Lock-in; the server must validate the provider's JWTs |

Common to all:
- **TLS mandatory** as soon as you leave localhost (gap #2) — otherwise credentials/tokens in clear text.
- Pass the token via **gRPC metadata** + a server-side **interceptor** that authenticates and injects the `user_id` into the `context`.
- **`user_id` isolation** enforced in every RPC (gap #3): never trust the client for the data scope.

## 6. Impacts on the contract (`proto`) and the future web

- The current `.proto` has **no notion of user**; identity will travel via the **metadata/token** (not via a `user_id` field in the messages — the client must not be able to spoof it).
- Index lists by `(user_id, title)` on the server/database side instead of `title` alone.
- For the **future web frontend**: expose the same service via **gRPC-Gateway** (REST/JSON) or **gRPC-Web**, generated from the `.proto` → a single backend, two clients (CLI + web).

## 7. Reasoned recommendation

**Recommendation: Option A — Managed PostgreSQL on a free tier (Neon or Supabase), gRPC server kept as the sole data gatekeeper, JWT auth (ideally via Supabase Auth to limit code), end-to-end TLS, `user_id` isolation.**

Why:

1. **Respects the existing architecture**: we keep the clean gRPC seam (`cmd/todo` → `proto` → `server` → persistence). Only `libs/storage` changes (JSON → SQL). The rest of the code moves little.
2. **Truly shared backend**: unlike Firestore (option B), the future web frontend will hit the **same gRPC server** (via gRPC-Gateway), so business logic and security stay centralized in one place.
3. **Multi-device / multi-user solved natively**: SQL transactions and constraints handle concurrency (gap #4), `user_id` handles isolation (gap #3).
4. **Free and durable**: Neon/Supabase offer durable free tiers sufficient for personal use.
5. **Continuity with the roadmap**: the README already announces "SQLite planned"; moving to remote Postgres is the natural evolution of the same relational model.

Recommended auth choice: **Supabase Auth (which issues JWTs)** if Supabase is chosen as the database, otherwise **homemade JWT + GitHub/Google OAuth** to prepare the web frontend without managing passwords.

To avoid for this goal: SQLite+Litestream (option C, not multi-writer) and Firestore (option B, short-circuits the targeted gRPC backend).

## 8. Security prerequisites (to address regardless of the option)

- [ ] Enable **TLS** on the gRPC transport (replace `insecure.NewCredentials()`).
- [ ] Add an **authentication interceptor** on the server side (token validation → `user_id` in the `context`).
- [ ] **Isolate data by `user_id`** in every RPC and every SQL query (parameterized queries).
- [ ] Manage **secrets** (DB connection string, JWT secret) via env variables / secret manager, outside the repo.
- [ ] Add **timeouts/retries** on the gRPC client side (absent today) for a remote network.
- [ ] Set up **backups** and a database restore plan.

---

*Analysis only — no implementation at this stage. Possible next step: a detailed implementation plan (migrating `libs/storage` to Postgres, SQL schema, auth interceptor, TLS, gRPC-Gateway).*
