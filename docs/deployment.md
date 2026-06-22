# Remote server deployment (Fly.io)

Phase 6 of [`implementation-plan.md`](./implementation-plan.md). Replaces the
local launchd daemon ([`daemon-setup.md`](./daemon-setup.md)): the gRPC server
runs on a free-tier PaaS (Fly.io), reachable publicly over TLS, and the CLI no
longer starts a server. See the target architecture in
[`target-architecture.md`](./target-architecture.md) §6.

## What ships where

- **Server** → container image built from the repo [`Dockerfile`](../Dockerfile)
  (only `./cmd/server`), deployed to Fly.io with [`fly.toml`](../fly.toml).
- **CLI** → stays on each user's machine, points at the remote endpoint via
  `TODO_SERVER_ENDPOINT` (see [`usage.md`](./usage.md)).

## Server environment variables

Set on the host (never committed — Fly.io secret manager / `[env]`):

| Variable | Role | How |
| --- | --- | --- |
| `DATABASE_URL` | Postgres connection string (Neon/Supabase) | `fly secrets set` |
| `SUPABASE_URL` | Project base URL → enables **JWKS / ES256 mode (Option B)**; server derives `{SUPABASE_URL}/auth/v1/.well-known/jwks.json` | `fly secrets set` |
| `SUPABASE_JWKS_URL` | Explicit JWKS endpoint override (full URL); takes precedence over `SUPABASE_URL` | `fly secrets set` (optional) |
| `SUPABASE_JWT_SECRET` | Validate JWTs against the **legacy HS256 secret (Option A)** | `fly secrets set` |
| `JWT_SIGNING_KEY` | Validate JWTs (home/dev mode only — wins over everything) | `fly secrets set` |
| `TLS_CERT_FILE` / `TLS_KEY_FILE` | End-to-end TLS to the container (optional; Fly terminates TLS at the edge by default) | `fly secrets set` / volume |
| `PORT` | Listen port; server binds `0.0.0.0:$PORT` | `fly.toml [env]` (default `50051`) |
| `TODO_STORAGE` | `postgres` on the host (default `json` is local-only) | `fly.toml [env]` |
| `TODO_LISTEN_ADDR` | Full `host:port` override (takes precedence over `PORT`) | optional |

`DATABASE_URL` and the JWT secret **never leave the server**; the CLI only knows
its endpoint and its own tokens (`target-architecture.md` §6.3).

## Supabase Auth (CLI ↔ Supabase ↔ server)

This is the **production identity** path. The CLI talks directly to Supabase
Auth (GoTrue) over HTTPS to sign up / log in / refresh, and the server only
**validates** the resulting JWTs — no Supabase client on the server. The server
supports two validation backends for Supabase tokens:

- **JWKS / ES256 mode (Option B, recommended target).** The server fetches the
  project's **public** signing key from the JWKS endpoint and verifies the
  asymmetric signature (ES256/RS256). **The project's active signing key can stay
  ECC/ES256** — no need to downgrade Supabase to HS256.
- **Legacy HS256 mode (Option A).** The server verifies the signature against the
  shared **legacy HS256 secret** (`SUPABASE_JWT_SECRET`). Kept as a fallback.

```
  todo signup/login ──HTTPS──▶ Supabase Auth (GoTrue)
        │                         emits access_token (sub=user.id, claim email)
        │ ◀── access + refresh JWT + refresh_token
        ▼
  credentials.json (0600)
        │ gRPC/TLS, metadata: authorization: Bearer <access JWT>
        ▼
  cmd/server ── Option B: JWKSVerifier({SUPABASE_URL}/auth/v1/.well-known/jwks.json) → ES256 → user_id
             ── Option A: HMACVerifier(SUPABASE_JWT_SECRET) → HS256 → user_id
             └─ just-in-time provisioning of users(id,email) from the signed token
```

### Environment variables

| Side | Variable | Role |
| --- | --- | --- |
| **CLI** | `SUPABASE_URL` | Project base URL, e.g. `https://<ref>.supabase.co`. The CLI appends the `/auth/v1` GoTrue prefix itself. |
| **CLI** | `SUPABASE_ANON_KEY` | Public anon / publishable key, sent as the `apikey` header on every GoTrue request. |
| **Server** | `SUPABASE_URL` | Same project base URL. Enables **JWKS / ES256 mode**; the server derives the JWKS endpoint `{SUPABASE_URL}/auth/v1/.well-known/jwks.json` and verifies the public-key signature (Option B). |
| **Server** | `SUPABASE_JWKS_URL` | Optional **full** JWKS URL that overrides the derived one (tests / non-default deployments). Takes precedence over `SUPABASE_URL`. |
| **Server** | `SUPABASE_JWT_SECRET` | Supabase project **legacy JWT secret (HS256)**, used by the server's `HMACVerifier` to verify HS256 access tokens (Option A fallback). |

So the server mapping is: **`SUPABASE_URL` + `TODO_STORAGE=postgres` +
`DATABASE_URL`** (JWKS / Option B), and the CLI mapping is **`SUPABASE_URL` +
`SUPABASE_ANON_KEY`**.

How the CLI selects its auth mode (`libs/clientauth/factory.go`,
`NewAuthenticatorFromEnv`), in this precedence:

1. **`SUPABASE_URL` + `SUPABASE_ANON_KEY` both set → Supabase Auth** (`SupabaseAuthenticator`, GoTrue HTTPS). Takes priority even if a leftover `JWT_SIGNING_KEY` is present, so the real IdP wins.
2. else **`JWT_SIGNING_KEY` set → local/home dev mode** (`DevAuthenticator`, HS256 minted locally).
3. else → clear error telling you which variables to set.

**Server-side selection** (`server/authconfig.go`, `AuthInterceptorFromEnv`), in
this precedence (most specific → fallback):

1. **`JWT_SIGNING_KEY` set → home/dev HS256** (`HMACVerifier`). Wins over everything so a developer can always force the home secret.
2. else **`SUPABASE_JWKS_URL` or `SUPABASE_URL` set → JWKS / ES256 mode** (`JWKSVerifier`, Option B). This is the production target now that Supabase projects sign with an active ES256 key.
3. else **`SUPABASE_JWT_SECRET` set → legacy HS256 mode** (`HMACVerifier`, Option A).
4. else → auth OFF (local default).

JWKS is placed **ahead** of the legacy HS256 secret because the project's active
signing key is ES256. **To force the legacy HS256 path, set only
`SUPABASE_JWT_SECRET`** (leave `SUPABASE_URL` and `SUPABASE_JWKS_URL` unset). For
either Supabase mode, **leave `JWT_SIGNING_KEY` unset on the server**. The JWKS
key set is fetched lazily on the first token verification, so server startup
never blocks on JWKS reachability.

### Manual steps (Supabase project — done once by a human)

These require a Supabase account and real secrets; they **cannot** be done in code.

1. **Create the project**: https://supabase.com → New project. Wait for it to finish provisioning.
2. **Enable Email/Password auth**: dashboard → **Authentication → Sign In / Providers → Email** → enable it.
   - For a smooth CLI flow, **disable "Confirm email"** (same Email provider settings). Otherwise `todo signup` returns a session with no access token until the user clicks the confirmation link, and the CLI will tell you to confirm then run `todo login`.
3. **Get the CLI values** (dashboard → **Project Settings → API**, or → **Data API**):
   - **Project URL** → `SUPABASE_URL` (e.g. `https://abcdefgh.supabase.co`).
   - **anon / public (publishable) key** → `SUPABASE_ANON_KEY`.
4. **Get the server value:**
   - **Option B (recommended — JWKS / ES256, the project key stays ES256):** no extra secret to copy. The server only needs the **same Project URL** as the CLI: set `SUPABASE_URL` on the server. It derives the JWKS endpoint `{SUPABASE_URL}/auth/v1/.well-known/jwks.json` and verifies the asymmetric signature with the project's **public** key. **The active signing key can remain ECC/ES256** (the Supabase default for new projects) — no downgrade needed.
   - **Option A (legacy HS256 fallback):** dashboard → **Project Settings → Authentication / JWT** → **JWT Keys**, "Legacy JWT secret" / "JWT Secret". Copy the **HS256 JWT secret** → `SUPABASE_JWT_SECRET`, and leave `SUPABASE_URL`/`SUPABASE_JWKS_URL` unset on the server. This requires the project to still issue HS256 tokens; if it was rotated to ES256-only, the legacy secret no longer validates tokens — use Option B instead.
5. **Where each value goes:**
   - `SUPABASE_URL`, `SUPABASE_ANON_KEY` → the **CLI** machine's environment (e.g. a local `.env` or shell export). These are not secrets in the strict sense (the anon key is public) but keep them out of the repo anyway.
   - `SUPABASE_URL` → also on the **server** environment for Option B (JWKS mode). It is the same public base URL, not a secret.
   - `SUPABASE_JWT_SECRET` → the **server** environment only (Fly.io secret / host env) for Option A. It must **never** reach the CLI or the repo.
6. If you ever paste a secret into a chat or log, **rotate it**.

### Live test sequence (Supabase signup → login → create → list)

Prerequisite — the **server** must be running with Postgres storage + a Supabase
verification mode. **Option B (recommended, project key stays ES256):**

```bash
# Server (host / Fly.io secrets):
export TODO_STORAGE=postgres
export DATABASE_URL="postgresql://...?sslmode=require"
export SUPABASE_URL="https://<ref>.supabase.co"   # enables JWKS / ES256 mode
# JWT_SIGNING_KEY and SUPABASE_JWT_SECRET must be UNSET so JWKS mode is selected.
# (apply migrations/0001_init.sql to the DB once, as above)
```

Option A (legacy HS256) instead — set `SUPABASE_JWT_SECRET` and leave
`SUPABASE_URL`/`SUPABASE_JWKS_URL` unset:

```bash
export TODO_STORAGE=postgres
export DATABASE_URL="postgresql://...?sslmode=require"
export SUPABASE_JWT_SECRET="<supabase legacy HS256 JWT secret>"
# JWT_SIGNING_KEY must be UNSET so Supabase HS256 mode is selected.
```

Then from the **CLI** machine, pointed at that server:

```bash
export SUPABASE_URL="https://<ref>.supabase.co"
export SUPABASE_ANON_KEY="<anon/publishable key>"
export TODO_SERVER_ENDPOINT="<server host:port>"
export TODO_TLS=1                      # validate the server certificate

todo signup --email you@example.com    # password at masked prompt; hits Supabase /auth/v1/signup
todo login  --email you@example.com    # if "Confirm email" was left on, confirm first, then login
todo create "groceries"                # Bearer <Supabase access JWT> → server validates with SUPABASE_JWT_SECRET
todo list                              # shows "groceries"
```

What to expect: signup/login store `credentials.json` (0600) with the Supabase
`access_token`/`refresh_token`; `UserID` = Supabase `user.id` (= JWT `sub`), so
server-side isolation by `user_id` and just-in-time `users` provisioning (from
the signed token's `email` claim) work without any extra wiring. When the access
token expires, the CLI refreshes it against
`/auth/v1/token?grant_type=refresh_token` (with refresh-token rotation) and
replays the call transparently.

## Manual steps (require accounts / secrets — to be done by a human)

These need external accounts and real secrets and **cannot** be performed in
code. Run them once:

1. **Managed Postgres** (Neon or Supabase): create a project, copy the
   connection string (`postgresql://...?sslmode=require`). Apply the schema:
   ```bash
   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0001_init.sql
   ```
2. **Supabase Auth** (for production JWT validation): create the project, enable
   Email/Password. **Option B (recommended):** set the Project URL →
   `SUPABASE_URL` on **both** the CLI and the server (server runs JWKS/ES256
   mode, the project key stays ES256), plus the anon key → `SUPABASE_ANON_KEY`
   (CLI). **Option A (legacy):** copy the legacy HS256 JWT secret →
   `SUPABASE_JWT_SECRET` (server) instead. Full step-by-step in
   [Supabase Auth](#supabase-auth-cli--supabase--server) below. (Local/dev
   alternative: a shared `JWT_SIGNING_KEY` — see `target-architecture.md` §5.)
3. **Fly.io app + deploy**:
   ```bash
   fly auth login
   fly launch --no-deploy            # pick a unique app name; keep fly.toml
   fly secrets set \
     DATABASE_URL="postgresql://...?sslmode=require" \
     SUPABASE_URL="https://<ref>.supabase.co"     # Option B (JWKS/ES256)
   # Option A instead: SUPABASE_JWT_SECRET="<supabase legacy HS256 jwt secret>"
   fly deploy
   ```
4. **Point the CLI at the deployed server**:
   ```bash
   export TODO_SERVER_ENDPOINT="<app-name>.fly.dev:443"
   export TODO_TLS=1                 # validate the public certificate
   todo login --email you@example.com
   todo create "my list"
   todo list
   ```

## Smoke test (validation)

After `fly deploy`, against the deployed endpoint:

```bash
TODO_SERVER_ENDPOINT="<app-name>.fly.dev:443" TODO_TLS=1 todo login --email you@example.com
TODO_SERVER_ENDPOINT="<app-name>.fly.dev:443" TODO_TLS=1 todo create "ci-smoke"
TODO_SERVER_ENDPOINT="<app-name>.fly.dev:443" TODO_TLS=1 todo list   # shows "ci-smoke"
```

Deploy to a **staging** app first, then promote to production.

## Notes

- **Cold start**: with `min_machines_running = 0` the free-tier machine sleeps
  when idle; the first request after a pause wakes it (a few seconds latency).
- **Secrets**: only via the Fly.io secret manager. `.gitignore` excludes
  `certs/`, `*.pem`, `*.key`, `*.crt`; CI runs a gitleaks secret scan.
- **Migrating existing local data**: see Phase 5 (`todo migrate`) in
  `implementation-plan.md`.
