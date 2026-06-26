# Usage Guide

Complete reference for all TodoList CLI commands.

## Prerequisites

The CLI talks to a **remote gRPC server** that stores your data in Postgres and
authenticates you against Supabase. For day-to-day use you need to:

1. Point the CLI at a server and configure authentication (see
   [installation.md](installation.md) for the full list of variables):
   ```bash
   export TODO_SERVER_ENDPOINT="<your-app>.fly.dev:443"
   export TODO_TLS=1
   export SUPABASE_URL="https://<project-ref>.supabase.co"
   export SUPABASE_ANON_KEY="<your-anon-key>"
   ```
2. Be logged in:
   ```bash
   todo signup    # first time only
   todo login
   ```

> **Local JSON mode (optional, dev/offline):** you can instead run a loopback
> server backed by a local `data.json` file (`TODO_STORAGE=json`, the default)
> with authentication disabled. Start it with `make dev`. This mode is meant for
> development and offline use; see [daemon-setup.md](daemon-setup.md) and
> [deployment.md](deployment.md).

## Quick Reference

| Command | Description |
|---------|-------------|
| `todo signup` | Create a remote account (interactive email + password) |
| `todo login` | Log in to your remote account |
| `todo logout` | Delete locally stored credentials |
| `todo list` | List all todo lists (shows non-completed item count) |
| `todo create <name> [items...]` | Create a new todo list |
| `todo show <name>` | Display non-completed items in a list |
| `todo show <name> -H` | Show full history (all items, including completed) |
| `todo add <name> <items...>` | Add new items to an existing list |
| `todo update <name>` | Edit an existing item (interactive) |
| `todo complete <name>` | Mark items as complete (soft-completion, interactive) |
| `todo delete <name>` | Delete an entire list (permanent) |
| `todo migrate` | Import local `data.json` lists into the remote Postgres store |

## Authentication

Your data is isolated per user: the server validates your JWT, derives your
`user_id` from it, and scopes every query to your account. You must be logged in
before the regular commands will work against a remote server.

### Sign Up

```bash
todo signup
```

Prompts for an email and a masked password (never passed as a flag, so it never
leaks into shell history or the process list). Against Supabase Auth this creates
a new account (email verification may apply depending on your project settings).

### Log In

```bash
todo login
# or pre-fill the email:
todo login --email you@example.com
```

On success the access/refresh tokens are written to
`~/.config/todolist/credentials.json` (file mode `0600`).

### Log Out

```bash
todo logout
```

Deletes the local credentials file. Your remote data is untouched.

## Commands

### List All Todo Lists

```bash
todo list
```

Shows all available todo lists with their non-completed item count. If no lists
exist, displays "No todo lists available."

### Create a New List

```bash
todo create <list-name> [item1] [item2] [item3...]
```

**Examples:**
```bash
# Empty list
todo create shopping

# List with items
todo create shopping "Buy milk" "Buy eggs" "Buy bread"
```

Use quotes around items with spaces. List names must be unique.

### Show a List

```bash
todo show <list-name>          # non-completed items only
todo show <list-name> -H       # full history (includes completed items)
```

**Example:**
```bash
todo show shopping
# Output:
# 1. [ ] Buy milk
# 2. [ ] Buy bread
```

By default `show` hides completed items. Add `-H` (history) to display every
item, including the ones that were marked complete.

### Add Items to a List

```bash
todo add <list-name> <item1> [item2] [item3...]
```

**Example:**
```bash
todo add shopping "Buy cheese" "Buy yogurt"
```

Items are appended to the end of the list. Shows the updated list after adding.

### Update an Existing Item

```bash
todo update <list-name>
```

**Interactive command:** Displays non-completed items, prompts for index, then
allows editing the item text with arrow keys (uses promptui). Press Enter to keep
current value or modify and save.

### Mark Items as Complete

```bash
todo complete <list-name>
```

**Interactive command:** Displays non-completed items with indices starting at 1.
Enter space-separated indices to mark as complete. Re-prompts if invalid indices
are entered.

> **Complete is soft-completion, not deletion.** Completing an item sets its
> `Completed` flag to true; the item is **kept** so you keep a history of what was
> done. Completed items are hidden from `todo show` and from the regular interactive
> lists, but you can see them again with `todo show <name> -H`. To remove data
> entirely, use `todo delete` on the whole list.

### Delete an Entire List

```bash
todo delete <list-name> [list-name2] [list-name3...]
```

**Example:**
```bash
todo delete shopping work personal
```

**Warning:** This deletes the entire list (including its history) permanently.

### Migrate Local Data to the Remote Store

```bash
todo migrate            # import data.json into Postgres for your account
todo migrate --dry-run  # preview what would be migrated, without writing
```

One-shot, idempotent import of your local `~/.config/todolist/data.json` lists
into the remote Postgres database, scoped to your authenticated account.

**Requirements:**
- You must be logged in (`todo login`) so the import is scoped to your `user_id`.
- `DATABASE_URL` must point at the same Postgres database the server uses
  (this is the one command that talks to Postgres directly).

Lists already present in Postgres are skipped, so re-running after an
interruption is safe. On success the local file is renamed to `data.json.bak`
(it is **never** deleted).

## Common Workflows

### Daily Tasks
```bash
todo login
todo create today "Review emails" "Team meeting" "Finish report"
todo show today
todo complete today              # Interactive: select items to mark done
todo show today -H               # Review what was completed
```

### Shopping List
```bash
todo create shopping "Milk" "Eggs" "Bread"
todo complete shopping           # Interactive: select items as bought
todo add shopping "Coffee"       # Add a forgotten item
todo delete shopping             # Done shopping — remove the whole list
```

### First-Time Setup From an Existing Local List
```bash
todo signup                      # create your remote account
todo login
todo migrate --dry-run           # preview the import
todo migrate                     # import your old data.json into Postgres
```

## Tips

- Use quotes around items with spaces: `"Buy milk"` not `Buy milk`
- Keep list names lowercase and simple: `shopping`, `work-tasks`
- Use `todo list` frequently to see all your lists
- Use `todo show <name> -H` to review completed items (they are kept, not deleted)

## Error Messages

**"connection refused" / call timeout**: The server is unreachable. Check
`TODO_SERVER_ENDPOINT`/`TODO_TLS`, or in local mode that the server is running
(`make dev`). Free-tier remote servers may take a few seconds to wake from a cold
start.

**"unauthenticated" / "No authentication mode configured"**: You are not logged
in, or `SUPABASE_URL`/`SUPABASE_ANON_KEY` are not set. Run `todo login` and check
your environment.

**"todo list does not exist"**: Check spelling or create with `todo create`

**"todo list already exists"**: Use a different name or delete the existing list first

## Data Location

Where your data lives depends on the mode:

- **Remote mode (recommended):** your todo lists live in the server's **Postgres
  database** (e.g. Neon), isolated per user. The only thing stored locally is
  `~/.config/todolist/credentials.json` (your tokens, mode `0600`). Back up /
  restore is handled by your Postgres provider, not by copying a local file.
- **Local JSON mode (dev/offline):** lists are stored in
  `~/.config/todolist/data.json` (selected by `TODO_STORAGE=json`, the default
  when no remote server is configured).

Back up the local JSON file (local mode only):
```bash
cp ~/.config/todolist/data.json ~/backup/todolist-$(date +%Y%m%d).json
```

To move local data into the remote store, use `todo migrate` (see above).

## Getting Help

- See [installation.md](installation.md) for setup and configuration
- See [deployment.md](deployment.md) for hosting the server remotely
- See [daemon-setup.md](daemon-setup.md) for the optional local self-hosted server
- In local mode: check server status with `make status` and logs with
  `tail -f /tmp/todoserver.log`
- Report bugs on GitHub
