# TodoList CLI

A command-line todo list manager written in **Go**. By default it runs **entirely
on your machine**: your lists are stored in a local JSON file, with no account and
no network required.

If you want to access the **same lists from several machines**, you can optionally
**self-host a small server** (PostgreSQL + authentication) and point the CLI at it.
See [Self-hosting](#self-hosting-sync-across-machines-optional). The project does
**not** ship a shared hosted service — any remote setup is one you run yourself.

## Features

- 📝 Create and manage multiple todo lists; add, update, and delete items
- ✅ Mark items complete — *soft-completion*: completed items are kept for history,
  not deleted (view them with `todo show <name> -H`)
- 💾 **Local by default** — data in a local JSON file, no account, works offline
- 🔄 **Optional self-hosted sync** — run your own server backed by PostgreSQL to use
  the same lists across machines
- 🔐 When self-hosting: authentication with per-user data isolation, TLS, and a REST
  gateway (gRPC-Gateway) for a future web front-end

## Quick Start (local)

**Platforms**: macOS, Linux, Windows · **Requires** Go 1.24+ to install from source.

```bash
# Install the CLI and the local server
go install github.com/oceane-vlt/todolist-cli/cmd/todo@latest
go install github.com/oceane-vlt/todolist-cli/cmd/server@latest
```

Make sure `~/go/bin` (or `%USERPROFILE%\go\bin` on Windows) is in your `PATH`.

The CLI talks to a small local server. From a clone of the repo, start it once,
then use the CLI:

```bash
make dev        # start the local server (127.0.0.1:50051, JSON storage, no account)

todo create shopping "Buy milk" "Buy eggs"
todo list
todo show shopping
todo add shopping "Buy bread"
todo complete shopping      # interactive; completed items are kept in history
todo delete shopping

make stop       # stop the local server when done
```

Data is stored locally in `~/.config/todolist/data.json` — no login, nothing leaves
your machine. On macOS you can have the local server start automatically on login;
see [docs/daemon-setup.md](docs/daemon-setup.md) (optional).

See [docs/usage.md](docs/usage.md) for the full command reference and
[docs/installation.md](docs/installation.md) for more details.

## Self-hosting (sync across machines, optional)

To use the same lists on several computers, you can run **your own** server backed
by a managed PostgreSQL database, with authentication so each account only sees its
own data. The CLI then points at your server instead of the local one.

This is an advanced, opt-in setup that **you host yourself** — there is no shared
service provided by this project, and no specific deployment details are committed
to this repo (every value is your own and stays on your machine).

➡️ Full step-by-step guide: **[docs/deployment.md](docs/deployment.md)**

In short, you would:

1. Provision a managed **PostgreSQL** database (e.g. a free tier) and apply the
   schema in [`migrations/`](migrations/).
2. Set up an **authentication** provider (Supabase Auth, email/password).
3. Deploy the **`server`** somewhere reachable (e.g. a free-tier PaaS like Fly.io),
   over **TLS**.
4. Configure the CLI with your own endpoint and credentials, then `todo signup` /
   `todo login`. Existing local lists can be imported once with `todo migrate`.

All the exact commands and environment variables are in
[docs/deployment.md](docs/deployment.md).

## Command reference (works the same in both modes)

```bash
todo list                                   # list all your todo lists
todo create shopping "Buy milk" "Buy eggs"  # create a list (optionally with items)
todo show shopping                          # show non-completed items
todo show shopping -H                       # show full history (incl. completed)
todo add shopping "Buy cheese"              # add items to a list
todo update shopping                        # edit an item (interactive)
todo complete shopping                      # mark items complete (interactive)
todo delete shopping                        # delete an entire list
```

When self-hosting, three extra commands manage your account:
`todo signup`, `todo login`, `todo logout` (and `todo migrate` to import local data).

See [docs/usage.md](docs/usage.md) for the complete reference.

## Documentation

- [Installation Guide](docs/installation.md) — install and (optionally) configure for a remote server
- [Usage Guide](docs/usage.md) — complete CLI command reference
- [Self-hosting / Deployment](docs/deployment.md) — run your own server (PostgreSQL + auth + TLS)
- [Daemon Setup](docs/daemon-setup.md) — auto-start the local server on macOS via launchd (optional)
- [Target Architecture](docs/target-architecture.md) — design / architecture decision record

## Project Structure

```
todolist-cli/
├── cmd/
│   ├── todo/          # CLI client commands
│   ├── server/        # gRPC server (local or self-hosted)
│   ├── gateway/       # REST gateway (gRPC-Gateway) for a future web front-end
│   └── notification/  # notification helper (see its README)
├── proto/             # Protocol Buffer definitions
├── libs/
│   ├── storage/       # Data persistence (JSON local / PostgreSQL remote)
│   ├── auth/          # Server-side JWT verification (self-hosted mode)
│   └── clientauth/    # CLI-side auth (Supabase / dev) and token storage
├── server/            # gRPC server internals (interceptors, config)
├── migrations/        # SQL schema for the PostgreSQL backend
├── scripts/           # Installation and service scripts
└── docs/              # Documentation
```

## Architecture

The CLI never talks to storage directly; it always goes through a gRPC server.

- **Local mode (default):** a lightweight `server` runs on `127.0.0.1`, stores data
  in `~/.config/todolist/data.json`, and (on loopback) runs with authentication off.
- **Self-hosted mode (optional):** the `server` runs on a remote host over TLS,
  stores data in **PostgreSQL**, validates a JWT from your auth provider, derives a
  `user_id`, and scopes every operation and SQL query to that user. A REST
  **gateway** (`cmd/gateway`) exposes the same RPCs as REST/JSON for a future web
  front-end.

The storage backend is selected by `TODO_STORAGE` (`json` by default, `postgres`
for the remote backend). See
[docs/target-architecture.md](docs/target-architecture.md) for the full design.

## Requirements

- Go 1.24 or later (to install from source)
- macOS, Linux, or Windows
- **Local mode:** nothing else
- **Self-hosting:** your own PostgreSQL database, an auth provider (Supabase), and a
  host for the server — see [docs/deployment.md](docs/deployment.md)
- Note: the optional launchd auto-start (local server) is macOS-only

## License

MIT License - see LICENSE file for details.
