# Installation Guide

This guide covers installing the TodoList CLI and using it. By **default the CLI
runs locally** (a small local server backed by a JSON file, no account, offline).
Connecting the CLI to a **self-hosted remote server** for multi-machine sync is an
optional, advanced step described at the end (and in full in
[deployment.md](deployment.md)).

## Prerequisites

- **Go 1.24 or later** (to install from source) - [Download Go](https://golang.org/dl/)
- **Supported platforms**: macOS, Linux, Windows
- **Local mode**: nothing extra.
- **Self-hosted mode (optional)**: your own reachable gRPC server endpoint and an
  auth provider (Supabase). See [deployment.md](deployment.md).

Verify Go installation:
```bash
go version  # Should show Go 1.24 or later
```

Make sure `~/go/bin` is in your PATH (or `%USERPROFILE%\go\bin` on Windows):
```bash
# macOS/Linux
echo $PATH | grep -q "$HOME/go/bin" && echo "✅ Go bin in PATH" || echo "❌ Add ~/go/bin to PATH"
```

If not in PATH, add this to your `~/.zshrc` or `~/.bashrc` (macOS/Linux):
```bash
export PATH="$HOME/go/bin:$PATH"
```

On Windows, add `%USERPROFILE%\go\bin` to your PATH environment variable.

## Installing the binaries

Install the CLI and the (local) server with `go install`:

```bash
go install github.com/oceane-vlt/todolist-cli/cmd/todo@latest    # CLI client
go install github.com/oceane-vlt/todolist-cli/cmd/server@latest  # gRPC server
```

The `gateway` binary (optional REST gateway, for a future web front-end) is only
needed when self-hosting:

```bash
go install github.com/oceane-vlt/todolist-cli/cmd/gateway@latest  # optional
```

Binaries are installed to `~/go/bin/` (`%USERPROFILE%\go\bin` on Windows):
- `todo` - the CLI client
- `server` - the gRPC server (local or self-hosted)
- `gateway` - the REST gateway (optional)

Verify installation:
```bash
which todo      # macOS/Linux: should show the path to todo
```

---

## Local mode (default)

By default the CLI talks to a small local server backed by a JSON file — no
account, no remote dependencies, fully offline. The server defaults to JSON
storage (`TODO_STORAGE=json`, data in `~/.config/todolist/data.json`), and when
bound to loopback it runs with **authentication off**.

Start a local server (from a clone of the repo):
```bash
make dev   # rebuilds, reinstalls, starts a local server on 127.0.0.1:50051 (JSON, auth off)
```

This will:
1. Stop any running server instances
2. Rebuild and reinstall the binaries
3. Start the server in the background
4. Log output to `/tmp/todoserver.log`

With the local server running and **no remote env vars set** (`TODO_SERVER_ENDPOINT`
unset → defaults to `127.0.0.1:50051`, `TODO_TLS` unset), the CLI talks to it
directly:
```bash
todo create mylist "first item"
todo list
```

Manage the local server:
```bash
make status        # or: pgrep -f server
make logs          # or: tail -f /tmp/todoserver.log
make stop          # or: pkill -f server
make dev           # restart after code changes
```

### Automatic local server on macOS (optional)

On macOS you can have the local server start automatically on login via launchd:
```bash
make install-service
```

See [daemon-setup.md](daemon-setup.md) for details and troubleshooting.

---

## Self-hosting: connecting the CLI to a remote server (optional)

If you want the same lists across several machines, you can run **your own** server
(PostgreSQL + authentication, deployed over TLS) and point the CLI at it. This is an
opt-in setup you host yourself; the full step-by-step (database, auth provider,
deploying the server) is in **[deployment.md](deployment.md)**.

Once your server is deployed, configure the CLI on each machine. Set these in your
shell profile (`~/.zshrc`, `~/.bashrc`) or a local `.env`:

| Variable | Required | Role |
| --- | --- | --- |
| `TODO_SERVER_ENDPOINT` | yes (remote) | `host:port` of the gRPC server, e.g. `<your-app>.fly.dev:443`. Defaults to `127.0.0.1:50051` when unset (local mode). |
| `TODO_TLS` | yes (remote) | Set to `1`/`true` to dial over TLS and validate the server certificate. Required for a public endpoint. |
| `SUPABASE_URL` | yes (remote) | Supabase project base URL, e.g. `https://<project-ref>.supabase.co`. The CLI appends the `/auth/v1` GoTrue prefix itself. |
| `SUPABASE_ANON_KEY` | yes (remote) | Public anon / publishable key, sent as the `apikey` header to Supabase Auth. |
| `TODO_TLS_CA_FILE` | optional | Path to a custom CA certificate (for a self-signed / private CA server). |
| `TODO_TLS_SERVER_NAME` | optional | Override the expected TLS server name (SNI / cert hostname). |
| `TODO_CALL_TIMEOUT` | optional | Per-call timeout (Go duration, e.g. `5s`). |

> Use your **own** values. Do not commit your real Supabase URL/anon key or your
> server hostname into the repo — they are personal to your deployment and belong
> in your local shell config or `.env`. The anon key is "public" but still keep it
> out of the repo. The `DATABASE_URL` and JWT secrets live **only on the server**,
> never on the CLI machine.

Example (your own public server with a valid certificate):
```bash
export TODO_SERVER_ENDPOINT="<your-app>.fly.dev:443"
export TODO_TLS=1
export SUPABASE_URL="https://<project-ref>.supabase.co"
export SUPABASE_ANON_KEY="<your-anon-key>"
```

Then create an account and verify:
```bash
# Create an account / sign in (password entered at a masked prompt)
todo signup --email you@example.com
todo login  --email you@example.com

# Use the CLI — data is stored remotely, scoped to your account
todo create test "First item" "Second item"
todo show test
todo list

# If you already had local data.json lists, import them once into the remote store:
todo migrate
```

Credentials are stored locally in `~/.config/todolist/credentials.json` (`0600`).
Run `todo logout` to delete them.

See [usage.md](usage.md) for the full command reference and
[deployment.md](deployment.md) to stand up the server, auth provider and database.

---

## Data Storage Location

Where your data lives depends on the mode:

- **Local mode (default)**: data is stored in `~/.config/todolist/data.json`. This
  directory is created automatically on first use.
- **Self-hosted mode (optional)**: your todo lists are stored in the server's
  **PostgreSQL** database, isolated per user. Nothing is kept on the CLI machine
  except your auth tokens in `~/.config/todolist/credentials.json` (`0600`).

The server decides which storage backend to use via the `TODO_STORAGE` environment
variable (`json` by default, `postgres` for the remote backend with a
`DATABASE_URL`). See [deployment.md](deployment.md).

## Troubleshooting

### "command not found: todo"

Your `~/go/bin` is not in your PATH. Add it:
```bash
export PATH="$HOME/go/bin:$PATH"
```

Make it permanent by adding the line to `~/.zshrc` or `~/.bashrc`.

### "connection refused" or "unavailable"

- **Local mode**: the local server isn't running — start it with `make dev` and
  check `make status`.
- **Self-hosted mode**: check `TODO_SERVER_ENDPOINT` points at the right `host:port`,
  that `TODO_TLS=1` is set for a public endpoint, and that your server is deployed
  and reachable. A free-tier machine may be asleep and take a few seconds to wake
  on the first request.

### "unauthenticated" / login errors (self-hosted mode)

- Verify `SUPABASE_URL` and `SUPABASE_ANON_KEY` are set and match your project.
- If Supabase has "Confirm email" enabled, confirm the email after `todo signup`,
  then run `todo login`.
- Make sure the server is configured with the matching Supabase verification mode
  (see [deployment.md](deployment.md)).

### Local server starts but crashes immediately

Check the logs:
```bash
tail -100 /tmp/todoserver.log
```

Common issues:
- **Port 50051 already in use**: another instance is running. Stop it with `make stop`
- **Permission denied on data file**: check permissions on `~/.config/todolist/`

### Can't find server binary

Reinstall:
```bash
make clean
make install
```

## Uninstallation

To remove the todolist CLI:

```bash
# Stop the local server (if you ran one)
make stop

# Uninstall binaries
make uninstall

# Remove local data and credentials (optional)
rm -rf ~/.config/todolist/
```

Removing `~/.config/todolist/` deletes local JSON data and your credentials; any
remote data in a self-hosted PostgreSQL database is unaffected.

## Next Steps

- Read the [Usage Guide](usage.md) to learn all available commands
- See [deployment.md](deployment.md) to self-host your own server
- Check [TODO.md](../TODO.md) for planned features

## Getting Help

If you encounter issues:
1. In local mode, check the logs: `tail -f /tmp/todoserver.log`
2. Verify your configuration (local: server running; self-hosted: `TODO_SERVER_ENDPOINT`, `TODO_TLS`, `SUPABASE_*`)
3. Confirm the server is reachable / running
4. Open an issue on GitHub with log output and error messages
