# Web UI plan — React + Vite front end for TodoList-CLI

> **Design document only (a plan).** This is **not** a production
> implementation: it settles the target for a web front end that reuses the
> **existing** gRPC backend through the REST/JSON gateway (`cmd/gateway`) and the
> **existing** Supabase Auth setup. Code excerpts (routes, `fetch` examples,
> folder layout) are **illustrative**. All secrets/values are **placeholders**.
>
> - **Status**: Proposed (builds on [`target-architecture.md`](./target-architecture.md) §3, [`grpc-gateway.md`](./grpc-gateway.md), [`deployment.md`](./deployment.md)).
> - **Date**: 2026-07-01.
> - **Decided choices**: front = **React + Vite** (SPA); auth = **Supabase Auth**
>   (Google OAuth + email/password), reusing the JWTs the server already
>   validates; data access = **REST calls to the gRPC-Gateway** (`cmd/gateway`),
>   so CLI and web share the **same** gRPC server → Postgres (Neon), isolated by
>   `user_id`; hosting = free tiers (static front on Vercel/Netlify/Cloudflare
>   Pages; gateway on Fly.io).
> - **Guiding principle** (unchanged from `target-architecture.md` §2): the gRPC
>   server stays the **sole guardian of the data**. The web UI adds **no new
>   trust boundary** — it is just a second client behind the same
>   `Authorization: Bearer <JWT>` seam as the CLI.

---

## 1. Target architecture of the web UI

The browser runs a static React+Vite SPA. It authenticates **directly** against
Supabase Auth (GoTrue) via the Supabase JS SDK (Google OAuth + email/password),
which manages the access/refresh token lifecycle in the browser. For data, the
SPA calls the **REST/JSON gateway** (`cmd/gateway`) over HTTPS, attaching
`Authorization: Bearer <access JWT>`. The gateway is a **pure relay**: it
forwards the bearer as gRPC metadata to the gRPC server, which **validates** the
token (ES256 via JWKS, Option B), derives `user_id`, and scopes every query.
**The gRPC server does not change** — it already validates the exact same
Supabase JWTs the CLI uses.

```
                          WEB UI TARGET (SPA reuses the existing backend)

  ┌────────────────────────────┐        signInWithOAuth('google')          ┌─────────────────────────────┐
  │  Browser — React + Vite SPA │  ───── signInWithPassword(email,pwd) ───▶ │  Supabase Auth (GoTrue, IdP) │
  │  (Supabase JS SDK)          │  ◀──── access JWT + refresh JWT ───────── │  emits sub=user.id, email    │
  │                             │        (SDK stores + auto-refreshes)      │  Google OAuth handled here   │
  │  token held by Supabase SDK │                                           └─────────────────────────────┘
  └──────────────┬──────────────┘
                 │  HTTPS  (fetch)
                 │  Authorization: Bearer <access JWT>
                 │  Content-Type: application/json
                 ▼
  ┌────────────────────────────────────────┐   gRPC metadata               ┌──────────────────────────────┐
  │  gRPC-Gateway  (cmd/gateway, on Fly.io)  │   authorization: Bearer <jwt> │   cmd/server  (gRPC, Fly.io)  │
  │  ┌────────────────────────────────────┐ │  ───────────────────────────▶ │  ┌─────────────────────────┐  │
  │  │ REST/JSON ↔ protojson              │ │                               │  │ auth interceptor:       │  │
  │  │ PURE RELAY of the Authorization    │ │                               │  │  JWKS/ES256 verify JWT  │  │
  │  │ header — NEVER validates the token │ │  ◀─────────────────────────── │  │  → user_id in context   │  │
  │  │ + CORS (to ADD, see §2/§6)         │ │        gRPC response          │  │  JIT provisioning users │  │
  │  └────────────────────────────────────┘ │                               │  └─────────────────────────┘  │
  └────────────────────────────────────────┘                               │  business RPCs scoped by      │
                                                                            │  user_id                      │
                                                                            └───────────────┬───────────────┘
                                                                                            │ pgx / SQL
                                                                                            ▼
                                                                            ┌──────────────────────────────┐
                                                                            │  Managed Postgres (Neon)     │
                                                                            │  lists/items WHERE user_id=… │
                                                                            └──────────────────────────────┘
```

Flow of one data call (e.g. list my todolists):

1. The SPA asks the Supabase SDK for the current session and reads
   `session.access_token`.
2. It issues `GET {VITE_API_BASE_URL}/v1/lists` with
   `Authorization: Bearer <access_token>`.
3. The gateway relays the header into gRPC metadata `authorization` and calls the
   upstream RPC. It does **not** inspect the token.
4. The server's interceptor validates the JWT (JWKS/ES256), extracts `sub`
   (= `user_id`) and the `email` claim (JIT provisioning), and scopes the SQL by
   `WHERE user_id = …`.
5. The gateway maps the gRPC response to JSON (protojson) and the gRPC status to
   an HTTP status; the SPA renders it. A **401** means the SPA should refresh the
   Supabase session (the SDK does this automatically) and retry.

---

## 2. What already exists and is reused vs. what to add

### 2.1 Reused as-is (grounded in the real repo)

| Building block | Where | What it already does |
| --- | --- | --- |
| **REST/JSON gateway** | `cmd/gateway/{main,handler,config}.go` | Hand-written adapter (`net/http` `ServeMux`, Go 1.22 method+pattern routing) exposing the 7 RPCs under `/v1` (§3). protojson (camelCase), 1 MiB body cap. **Works today** (`go build`/`go test` green). |
| **Bearer relay** | `cmd/gateway/handler.go` (`callContext`) | Reads the HTTP `Authorization` header, forwards it verbatim as gRPC metadata `authorization`. **Never validates** the token. |
| **gRPC→HTTP error mapping** | `cmd/gateway/handler.go` (`writeError`/`httpStatusFromGRPC`) | `Unauthenticated`→401, `NotFound`→404, `AlreadyExists`/`Aborted`→409, `InvalidArgument`/`FailedPrecondition`/`OutOfRange`→400, `PermissionDenied`→403, `Unavailable`→503, `DeadlineExceeded`→504, `Unimplemented`→501, `ResourceExhausted`→429, `Canceled`→499, default→500. Error body: `{"code":"<GRPC_CODE>","message":"…"}`. |
| **Server JWT validation** | `server/authinterceptor.go`, `server/authconfig.go` | Validates the JWT (JWKS/ES256 Option B, or HS256 fallbacks), derives `user_id` from `sub`, injects it via `storage.WithUserID`, and does **JIT provisioning** of `users(id,email)` from the token's `email` claim. Identity comes **only** from the token, never from a client field (`target-architecture.md` §4.1). |
| **Supabase Auth (IdP)** | Supabase project (external) + `libs/clientauth/supabaseauth.go` (CLI side) | The CLI already uses GoTrue over HTTPS (`/auth/v1/*`). The **same** project issues the JWTs the web SPA will use. The server already trusts these tokens — **no server change**. |
| **Server deployment** | `Dockerfile` (builds only `./cmd/server`), `fly.toml`, `docs/deployment.md` | `cmd/server` is the deployable unit today (Fly.io, TLS at the edge, Postgres/Neon, Supabase JWKS). |

### 2.2 To add for a browser client

| To add | Why | Where | Scope |
| --- | --- | --- | --- |
| **CORS on the gateway** | **Confirmed absent**: `cmd/gateway` sets no CORS headers, has no `OPTIONS`/preflight handler and no middleware. A browser on a different origin (front on Vercel/Netlify/CF Pages) → gateway on Fly.io will fail the preflight. | `cmd/gateway` (new middleware wrapping the mux) | Backend, small. Allow the front origin(s); allow headers `Authorization`, `Content-Type`; allow methods `GET, POST, PUT, PATCH, DELETE, OPTIONS`; answer preflight `OPTIONS` with 204. Origin from an env var (§6). |
| **Deploy the gateway** | Only `cmd/server` is deployed today. The `Dockerfile` builds **only** `./cmd/server`; `grpc-gateway.md` §5 states the gateway's public exposure/inbound TLS is not yet done. The SPA needs a public HTTPS gateway URL. | new Fly.io app (or a second process/image), `TODO_GATEWAY_ADDR=0.0.0.0:$PORT`, `TODO_SERVER_ENDPOINT=<server>`, `TODO_GATEWAY_UPSTREAM_TLS=1` | Deployment, no business-logic change. |
| **The React+Vite SPA** | The front end itself does not exist (`web/`, `frontend/` absent). | new `web/` directory (§5) | New front-end app. |
| **Supabase: enable Google provider + web redirect URLs** | Email/password already usable; Google OAuth and the SPA's redirect URL must be configured in Google Cloud + Supabase (§4). | Supabase dashboard + Google Cloud (manual, human) | Config only, placeholders. |

> **Not changed**: the 7 RPCs, the proto contract, the server's auth/validation,
> the `user_id` isolation, the JIT provisioning, and the JWT format. The web UI
> is purely additive.

---

## 3. Real REST endpoints exposed by the gateway, and how the SPA calls them

The routes below are the **actual** ones registered in
`cmd/gateway/handler.go` (verified against source), all under `/v1`.
(De)serialization is **protojson** canonical proto3 JSON: field names are
**camelCase** (`dueDate`, `itemIndexes`, `newTitle`, `itemIndex`),
`EmitUnpopulated=true` on responses, `DiscardUnknown` on decode, request body
capped at 1 MiB. For the item routes, the **`{title}` in the path overrides**
any `title` in the body.

| Method + path | RPC | Request body (protojson) | Notes |
| --- | --- | --- | --- |
| `POST /v1/lists` | `CreateTodoList` | `{"title":"courses","item":[{"title":"lait","priority":"high","dueDate":"2026-07-10"}]}` | Note the create field is **`item`** (proto field 3), not `items`. |
| `GET /v1/lists` | `GetTodoLists` | — | Response `{"lists":[{"title":"courses","size":3}]}`. |
| `DELETE /v1/lists` | `DeleteTodoList` | `{"title":["courses","work"]}` | `title` is a **repeated** string (list of titles to delete). |
| `GET /v1/lists/{title}/items` | `ShowTodoListItems` | — | Response `{"items":[{"title":"lait","description":"","completed":false,"dueDate":"","priority":""}]}` (`EmitUnpopulated`). |
| `PUT /v1/lists/{title}/items` | `UpdateTodoList` | `{"items":[{"title":"lait"},{"title":"pain"}]}` | Replaces the list's items; **`items`** (proto field 2) here. Path title wins. |
| `PATCH /v1/lists/{title}/items` | `UpdateTodoListItem` | `{"itemIndex":0,"newTitle":"lait entier"}` | Renames one item by index. Path title wins. |
| `DELETE /v1/lists/{title}/items` | `DeleteTodoListItems` | `{"itemIndexes":[0,2]}` | `itemIndexes` = indexes to delete. Path title wins. |

Every request carries `Authorization: Bearer <access JWT>`; writes also send
`Content-Type: application/json`. Illustrative `fetch` wrapper (SPA side):

```ts
// ILLUSTRATIVE — not production code.
// getAccessToken() returns supabase.auth.getSession()'s access_token.
async function api(path: string, init: RequestInit = {}) {
  const token = await getAccessToken();
  const res = await fetch(`${import.meta.env.VITE_API_BASE_URL}${path}`, {
    ...init,
    headers: {
      ...(init.body ? { "Content-Type": "application/json" } : {}),
      Authorization: `Bearer ${token}`,
      ...init.headers,
    },
  });
  if (res.status === 401) {
    // Session expired: let the Supabase SDK refresh, then retry once.
    await supabase.auth.refreshSession();
    return api(path, init);
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message ?? `HTTP ${res.status}`); // {code,message} envelope
  }
  return res.status === 204 ? null : res.json();
}

// Examples:
// const { lists } = await api("/v1/lists");
// await api("/v1/lists", { method: "POST", body: JSON.stringify({ title, item: [] }) });
// const { items } = await api(`/v1/lists/${encodeURIComponent(title)}/items`);
// await api(`/v1/lists/${encodeURIComponent(title)}/items`,
//           { method: "PATCH", body: JSON.stringify({ itemIndex, newTitle }) });
```

> **Note on route granularity.** The gateway's item routes are coarse-grained:
> "complete an item" or "update fields" are not distinct REST verbs. Given the
> current RPCs, the SPA composes CRUD from: `PUT` (replace the whole item set),
> `PATCH` (rename one item by index), `DELETE …/items` (remove by index). Marking
> an item complete via the web (like ticking it in the CLI's browser) maps to
> a `PUT` that resubmits the item set with `completed:true` on the target item.
> No proto/RPC change is in scope for v1.

---

## 4. Authentication: Supabase (Google OAuth + email/password)

The SPA uses `@supabase/supabase-js`. The Supabase client is created from the
public project URL and anon key (both browser-safe):

```ts
// ILLUSTRATIVE. VITE_* come from build-time env (placeholders, §6).
import { createClient } from "@supabase/supabase-js";
export const supabase = createClient(
  import.meta.env.VITE_SUPABASE_URL,
  import.meta.env.VITE_SUPABASE_ANON_KEY,
);
```

- **Email/password**: `supabase.auth.signUp({ email, password })` /
  `supabase.auth.signInWithPassword({ email, password })`. This is the **same**
  GoTrue email/password path the CLI already uses (`libs/clientauth`), so no new
  server trust is introduced.
- **Google**: `supabase.auth.signInWithOAuth({ provider: "google", options: { redirectTo } })`.
  The browser is redirected to Google, then back to Supabase, then to the SPA's
  `redirectTo`; the SDK finalizes the session on return.
- **Token lifecycle**: the Supabase JS SDK **owns** token storage and refresh in
  the browser (by default in `localStorage`, auto-refresh on). The SPA never
  hand-rolls token storage; it reads `access_token` from
  `supabase.auth.getSession()` when calling the API and listens to
  `supabase.auth.onAuthStateChange` for login/logout.

### 4.1 What the user configures (placeholders — no personal values in this repo)

**Google Cloud** (once, by a human):
1. Create/choose a project → **APIs & Services → OAuth consent screen** (external),
   fill app name/support email.
2. **Credentials → Create OAuth client ID → Web application**.
3. **Authorized JavaScript origins**: your front origin(s), e.g.
   `https://<your-app>.vercel.app` (and `http://localhost:5173` for dev).
4. **Authorized redirect URI**: the Supabase callback,
   `https://<ref>.supabase.co/auth/v1/callback`.
5. Copy the **Client ID** and **Client secret** (placeholders — kept in Supabase,
   never in the repo).

**Supabase** (once, by a human):
1. **Authentication → Sign In / Providers → Google**: enable, paste the Google
   **Client ID** + **Client secret**.
2. **Authentication → Sign In / Providers → Email**: enabled (already used by the
   CLI). For a smooth web signup, decide on "Confirm email" per product need.
3. **Authentication → URL Configuration**: set **Site URL** to the front origin
   and add **Redirect URLs** for every environment (e.g.
   `https://<your-app>.vercel.app/**`, `http://localhost:5173/**`).
4. **Project Settings → API**: the **Project URL** → `VITE_SUPABASE_URL`, the
   **anon/publishable key** → `VITE_SUPABASE_ANON_KEY` (§6).

### 4.2 The server does not change

The JWTs Google-OAuth and email/password sessions produce are the **same**
Supabase-issued access tokens (`sub` = user id, `email` claim) the server already
validates via JWKS/ES256 (`server/authconfig.go` Option B). So enabling Google
adds **zero** server-side work: same signing key, same JWKS endpoint
(`{SUPABASE_URL}/auth/v1/.well-known/jwks.json`), same `user_id` isolation and
JIT provisioning. Google users are provisioned just-in-time exactly like
email/password users.

---

## 5. Proposed React + Vite app structure

New top-level **`web/`** directory (sibling of `cmd/`, `server/`, `docs/`;
neither `web/` nor `frontend/` exists today). Illustrative layout:

```
web/
├── index.html
├── package.json
├── vite.config.ts
├── .env.example              # VITE_* placeholders (no real values committed)
├── src/
│   ├── main.tsx              # React root, router
│   ├── App.tsx               # routes + auth guard
│   ├── lib/
│   │   ├── supabase.ts       # createClient(VITE_SUPABASE_URL, VITE_SUPABASE_ANON_KEY)
│   │   └── api.ts            # fetch wrapper: base URL, Bearer, 401→refresh→retry, {code,message}
│   ├── auth/
│   │   ├── AuthProvider.tsx  # session context via supabase.auth.onAuthStateChange
│   │   └── RequireAuth.tsx   # redirects to /login when no session
│   ├── pages/
│   │   ├── Login.tsx         # Google button + email/password form
│   │   ├── AuthCallback.tsx  # handles the OAuth return (redirectTo)
│   │   ├── Lists.tsx         # GET /v1/lists; create (POST), delete (DELETE /v1/lists)
│   │   └── ListDetail.tsx    # GET …/items; add/complete/rename/delete items
│   └── components/           # ListCard, ItemRow, forms, error/loading states
```

- **Pages/views**: `Login` (Google + email/password), `Lists` (all todolists +
  create/delete), `ListDetail` (a list's items: add / complete / rename / delete).
- **State & API layer**: a thin `api.ts` wrapper (§3) centralizes the base URL,
  the `Authorization` header, the 401→refresh→retry, and the `{code,message}`
  error envelope. A small data-fetching layer (e.g. React Query or hand-rolled
  hooks) caches lists/items and revalidates after mutations.
- **Token storage**: **delegated to the Supabase SDK** — the SPA does not persist
  tokens itself. `RequireAuth` gates routes on the presence of a session.
- **Routing**: `/login`, `/auth/callback`, `/` (lists), `/lists/:title`.

---

## 6. Free hosting

| Component | Host (free tier) | Notes |
| --- | --- | --- |
| **Static front (SPA)** | **Vercel** / **Netlify** / **Cloudflare Pages** | Build `vite build` → static `dist/`. SPA fallback to `index.html`. Set the `VITE_*` env at build time. |
| **Gateway** | **Fly.io** | New app/process running `cmd/gateway` with CORS + public HTTPS (§2.2). TLS terminated at the Fly edge like `cmd/server`. |
| **gRPC server** | **Fly.io** (already the deploy target) | Unchanged (`Dockerfile` builds `./cmd/server`, `fly.toml`, Postgres/Neon, Supabase JWKS). |
| **Postgres** | **Neon** (free) | Unchanged. |
| **Auth** | **Supabase** (free) | Unchanged; add Google provider + web redirect URLs (§4.1). |

**Front build-time env** (Vite exposes only `VITE_`-prefixed vars to the client;
all values below are **public-safe** and shown as placeholders):

| Variable | Placeholder | Role |
| --- | --- | --- |
| `VITE_SUPABASE_URL` | `https://<ref>.supabase.co` | Supabase project URL for the JS SDK. |
| `VITE_SUPABASE_ANON_KEY` | `<anon/publishable key>` | Public anon key (safe in the browser; RLS/JWT enforce access). |
| `VITE_API_BASE_URL` | `https://<your-gateway>.fly.dev` | Base URL of the deployed gateway (SPA prefixes `/v1/...`). |

**Gateway env** (Fly.io, from `cmd/gateway/config.go` + the new CORS var):

| Variable | Placeholder | Role |
| --- | --- | --- |
| `TODO_GATEWAY_ADDR` | `0.0.0.0:$PORT` | Public bind (default `127.0.0.1:8080` is loopback-only — must be overridden to serve a browser). |
| `TODO_SERVER_ENDPOINT` | `<server-app>.fly.dev:443` | Upstream gRPC server. |
| `TODO_GATEWAY_UPSTREAM_TLS` | `1` | Dial the upstream over TLS (prod). |
| `TODO_GATEWAY_CORS_ORIGIN` *(to add)* | `https://<your-app>.vercel.app` | Allowed browser origin(s) for the CORS middleware (§2.2). Name illustrative. |

---

## 7. Incremental phases

Each phase: **objective**, **what changes**, **validation**. Keep the repo green
at every step (the existing CLI/server/gateway builds and tests must stay
passing; the front lives under `web/` and does not affect Go build/test).

- **Phase 0 — Scaffold + gateway CORS.**
  - *Objective*: a runnable empty SPA and a browser-callable gateway.
  - *Changes*: create `web/` (Vite + React + TS, `.env.example`); add CORS
    middleware + `OPTIONS` handling to `cmd/gateway` (allowed origin from env).
  - *Validation*: `vite dev` serves a page; from the browser, a preflight
    `OPTIONS /v1/lists` and a `GET /v1/lists` succeed cross-origin against a local
    gateway (dev server default-accepts, no token needed); `go build ./... && go
    test ./...` still green; gateway CORS covered by a handler test.

- **Phase 1 — Supabase auth (Google + email/password).**
  - *Objective*: users can sign in and the SPA holds a session.
  - *Changes*: `lib/supabase.ts`, `AuthProvider`, `RequireAuth`, `Login`,
    `AuthCallback`; enable the Google provider + redirect URLs in Supabase/Google
    Cloud (manual).
  - *Validation*: email/password login and Google login both land back in the SPA
    with a session; `getSession()` returns an `access_token`; logout clears it.

- **Phase 2 — Read lists.**
  - *Objective*: display the signed-in user's todolists.
  - *Changes*: `lib/api.ts` (Bearer + 401 refresh/retry); `Lists` page calling
    `GET /v1/lists`.
  - *Validation*: against a gateway+server with Supabase JWKS enabled, the SPA
    shows the same lists the CLI shows for that account; a tampered/expired token
    yields 401 → the SDK refreshes and the call retries.

- **Phase 3 — CRUD.**
  - *Objective*: full list/item management from the web.
  - *Changes*: create/delete lists (`POST`/`DELETE /v1/lists`); `ListDetail` with
    add/complete/rename/delete items (`PUT` / `PATCH` / `DELETE …/items`),
    remembering the coarse-grained mapping in §3.
  - *Validation*: changes made in the web UI are visible in the CLI for the same
    user (and vice-versa); index-based edits behave correctly (path title wins).

- **Phase 4 — Deploy.**
  - *Objective*: public, free-tier hosting.
  - *Changes*: deploy `cmd/gateway` to a Fly.io app (public bind, CORS origin set
    to the front URL, upstream TLS); build+deploy the SPA to Vercel/Netlify/CF
    Pages with the `VITE_*` env; finalize Supabase redirect URLs for prod.
  - *Validation*: from the production SPA URL, login (Google + email/password),
    read, and CRUD all work end-to-end; CORS restricted to the front origin;
    no secret shipped in the front bundle.

---

## 8. Security / points of attention

- **Anon key is public**: `VITE_SUPABASE_ANON_KEY` is meant to ship in the browser
  bundle. Access control does **not** rely on it being secret — it relies on the
  Supabase-issued **JWT** and the server's `user_id` scoping. Do not treat it as a
  credential, but keep it out of the repo (env at build time).
- **CORS restricted to the front origin**: the gateway must allow **only** the
  known front origin(s), not `*`, especially since requests carry an
  `Authorization` header. Allow methods `GET/POST/PUT/PATCH/DELETE/OPTIONS` and
  headers `Authorization, Content-Type`.
- **No `DATABASE_URL` (or any server secret) in the front**: only `VITE_*`
  (public) values reach the browser. `DATABASE_URL`, `SUPABASE_JWT_SECRET`, etc.
  live **only** on the server/gateway hosts (`deployment.md`,
  `target-architecture.md` §6.3).
- **HTTPS/TLS everywhere**: SPA→gateway over HTTPS (Fly edge TLS), gateway→server
  over TLS (`TODO_GATEWAY_UPSTREAM_TLS=1`), browser→Supabase over HTTPS. No bearer
  token ever travels in clear text.
- **Token expiry / refresh**: the SDK auto-refreshes; the API wrapper treats a
  **401** as "refresh and retry once" (mirrors the CLI's `Unauthenticated`→refresh
  path). On persistent 401, route to `/login`.
- **The gateway never validates tokens**: it is a relay. All authorization is on
  the gRPC server (JWKS/ES256). A misconfigured/oversharing gateway must not be
  able to bypass this — it simply cannot mint or alter `user_id`, which is derived
  from the signed token server-side.
- **XSS caveat of `localStorage` tokens**: the Supabase SDK stores tokens in
  `localStorage` by default, so a strong CSP and dependency hygiene matter (an XSS
  could read the token). Cookie-based storage / SSR hardening is **out of scope
  for v1** (§9).

---

## 9. Explicitly out of scope for v1

- **SSR / SSG / Next.js** — the front is a static client-only SPA.
- **Offline mode / local cache persistence / PWA**.
- **Web push / notifications** (the repo's `cmd/notification` is unrelated to the
  web UI here).
- **Cookie/`httpOnly` token storage, CSRF hardening beyond CORS** — kept as a
  known caveat (§8), not implemented in v1.
- **Advanced theming / i18n / accessibility polish** beyond baseline.
- **New RPCs or proto changes** (e.g. a dedicated "complete item" or partial-field
  update endpoint) — v1 composes CRUD from the existing 7 RPCs.
- **Realtime (Supabase Realtime / websockets), sharing/collaboration, drag-and-drop
  reordering** beyond what index-based item ops allow.
- **gRPC-Web** as an alternative transport — the plan uses the REST gateway.

---

*Plan / design document only — no production implementation. Routes, `fetch`
snippets and the folder tree are illustrative; all URLs/keys are placeholders.
Next step (out of scope here): implement Phase 0.*
