# TodoList CLI

A modern **command-line todo list manager** built with **Go** and **gRPC**, with
**remote sync**: your lists live in a managed **PostgreSQL** database (Neon) behind
a gRPC server, so you can sign in from several machines and see the same data.

Authentication is handled by **Supabase Auth** (email/password): the CLI obtains a
JWT, the server validates it and **isolates each user's data by `user_id`**. The
server is **deployable to a free-tier PaaS** (Fly.io) and reachable over TLS. A
**REST gateway** (gRPC-Gateway) is included so a future web front-end can reuse the
same backend.

A **local JSON mode** is also available for offline/dev use (no account, data in a
local file).

## Features

- 🔄 Remote sync — lists stored in managed **PostgreSQL** (Neon) via a gRPC server
- 🔐 Authentication with **Supabase Auth** (email/password), JWT validated server-side
- 👥 Per-user data isolation (each account sees only its own lists)
- 💻 Multi-machine access — log in from any device, same data
- ☁️ Server deployable to a free-tier PaaS (**Fly.io**) over **TLS**
- 🌐 REST gateway (gRPC-Gateway) for a future web front-end
- 📝 Create and manage multiple todo lists; add, update, delete items
- ✅ Mark items as complete (soft-completion — completed items are kept for history,
  not deleted)
- 💾 Optional **local JSON mode** for offline/dev use

## Quick Start

### Installation

**Platforms**: macOS, Linux, Windows

```bash
# Install the CLI
go install github.com/oceane-vlt/todolist-cli/cmd/todo@latest
```

This installs the `todo` binary to `~/go/bin/` (or `%USERPROFILE%\go\bin` on
Windows). Make sure that directory is in your `PATH`. You usually do **not** need
to install the server yourself — point the CLI at an already-deployed server (see
below). To run your own server, also `go install .../cmd/server` (and
`.../cmd/gateway` for REST).

See [docs/installation.md](docs/installation.md) for the full installation and
configuration guide.

### Using a remote server (recommended)

Point the CLI at the server and at your Supabase project, then create an account
and log in:

```bash
# Where the gRPC server lives, and how to reach Supabase Auth
export TODO_SERVER_ENDPOINT="<your-app>.fly.dev:443"
export TODO_TLS=1                                   # validate the server certificate
export SUPABASE_URL="https://<project-ref>.supabase.co"
export SUPABASE_ANON_KEY="<your-anon-key>"

# Create an account / sign in (password entered at a masked prompt)
todo signup --email you@example.com
todo login  --email you@example.com

# Use the CLI — data is stored remotely, scoped to your account
todo create shopping "Buy milk" "Buy eggs"
todo list
todo show shopping

# If you have existing local data.json lists, import them once:
todo migrate

# Sign out (removes locally stored credentials)
todo logout
```

Don't have a server yet? See [docs/deployment.md](docs/deployment.md) to deploy
one to Fly.io and provision Supabase + Postgres (Neon).

### Local JSON mode (offline / dev)

For offline use or development you can run a local server backed by a JSON file —
no account required. The server defaults to JSON storage
(`~/.config/todolist/data.json`), and when bound to loopback it runs with auth off:

```bash
make dev        # start a local server (127.0.0.1:50051, JSON storage, auth off)
todo create mylist "first item"
todo list
make stop       # stop the local server
```

On macOS you can run that local server automatically via launchd — see
[docs/daemon-setup.md](docs/daemon-setup.md) (optional; the remote deployment above
is the recommended setup).

### Basic Usage Example

```bash
# List all todo lists
todo list

# Create a new list with items
todo create shopping "Buy milk" "Buy eggs" "Buy bread"

# Show a specific list
todo show shopping

# Add items to an existing list
todo add shopping "Buy cheese" "Buy butter"

# Mark items as complete (interactive; completed items stay in history)
todo complete shopping

# Delete an entire list
todo delete shopping
```

See [docs/usage.md](docs/usage.md) for the complete CLI reference.

## Documentation

- [Installation Guide](docs/installation.md) - Installation and client configuration
- [Usage Guide](docs/usage.md) - Complete CLI command reference
- [Deployment](docs/deployment.md) - Deploy the server remotely (Fly.io + Supabase + Postgres)
- [Daemon Setup](docs/daemon-setup.md) - Run a local self-hosted server via launchd (macOS, optional)
- [Target Architecture](docs/target-architecture.md) - Architecture decision record

## Project Structure

```
todolist-cli/
├── cmd/
│   ├── todo/          # CLI client commands
│   ├── server/        # gRPC server
│   ├── gateway/       # REST gateway (gRPC-Gateway) for a future web front-end
│   └── notification/  # notification helper (see its README)
├── proto/             # Protocol Buffer definitions
├── libs/
│   ├── storage/       # Data persistence (JSON local / Postgres remote)
│   ├── auth/          # Server-side JWT verification
│   └── clientauth/    # CLI-side auth (Supabase / dev) and token storage
├── server/            # gRPC server internals (interceptors, config)
├── migrations/        # SQL schema for the Postgres backend
├── scripts/           # Installation and service scripts
└── docs/              # Documentation
```

## Architecture

The CLI never talks to the database directly; it always goes through the gRPC
server, which is the single guardian of the data.

- **CLI Client** (`cmd/todo`): authenticates with Supabase Auth, stores its tokens
  locally (`~/.config/todolist/credentials.json`, `0600`), and attaches the access
  JWT as gRPC metadata on each call.
- **gRPC Server** (`cmd/server`): validates the JWT, derives the `user_id`, and
  scopes every operation and SQL query to that user. Talks to **Postgres** (Neon)
  over `pgx`. Runs over **TLS** when deployed remotely.
- **Storage** (`libs/storage`): selected by `TODO_STORAGE` — `postgres` (remote,
  multi-user) or `json` (local file, default for dev).
- **REST Gateway** (`cmd/gateway`): exposes the same RPCs as REST/JSON for a future
  web front-end, reusing the one backend.
- **Protocol Buffers** (`proto/`): defines the contract between client and server.

Remote data is stored in PostgreSQL, isolated per user. In local JSON mode, data is
stored in `~/.config/todolist/data.json`. See
[docs/target-architecture.md](docs/target-architecture.md) for the full design.

## Requirements

- Go 1.24 or later (to install the CLI from source)
- macOS, Linux, or Windows
- **For remote use**: a reachable gRPC server endpoint (`TODO_SERVER_ENDPOINT`) and
  a Supabase project (`SUPABASE_URL`, `SUPABASE_ANON_KEY`) to authenticate against —
  see [docs/deployment.md](docs/deployment.md)
- **For local JSON mode**: nothing extra (data in a local file)
- Note: the optional launchd daemon (automatic local server startup) is macOS-only

## License

MIT License - see LICENSE file for details.
