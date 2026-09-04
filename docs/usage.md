# Usage Guide

Complete reference for all TodoList CLI commands.

## Prerequisites

By **default the CLI runs locally**: it talks to a small local gRPC server backed
by a JSON file, with no account. Start the local server once, then use the
commands below:

```bash
make dev   # start a local server on 127.0.0.1:50051 (JSON storage, auth off)
```

With no remote environment variables set, the CLI dials `127.0.0.1:50051`
automatically — nothing else to configure. See [installation.md](installation.md).

> **Self-hosted mode (optional):** to sync the same lists across machines you can
> point the CLI at **your own** remote server (PostgreSQL + authentication). Then
> you also set `TODO_SERVER_ENDPOINT`, `TODO_TLS`, `SUPABASE_URL`,
> `SUPABASE_ANON_KEY` and log in. The account commands (`signup`/`login`/`logout`)
> and `migrate` only apply in that mode. Full setup in
> [deployment.md](deployment.md).

## Quick Reference

| Command | Description |
|---------|-------------|
| `todo signup` | Create a remote account (interactive email + password) |
| `todo login` | Log in to your remote account |
| `todo logout` | Delete locally stored credentials |
| `todo list` | List all todo lists (shows non-completed item count) |
| `todo create <name> [items...]` | Create a new todo list |
| `todo show <name>` | Browse a list: read descriptions, tick items, complete them |
| `todo show <name> --plain` | Static rendering (automatic when piped) |
| `todo show <name> --details` | Static rendering with descriptions printed inline |
| `todo show <name> -H` | Show full history (all items, including completed) |
| `todo add <name> <items...>` | Add new items to an existing list |
| `todo add <name> <item> -d "..."` | Add one item with a longer description |
| `todo update <name>` | Edit an item's title and description (interactive) |
| `todo delete <name>` | Delete an entire list (permanent) |
| `todo migrate` | Import local `data.json` lists into the remote Postgres store |

## Authentication (self-hosted mode only)

> These commands only apply when you point the CLI at **your own remote server**.
> In local mode (the default) there is no account and no login.

When self-hosting, your data is isolated per user: the server validates your JWT,
derives your `user_id` from it, and scopes every query to your account. You must be
logged in before the regular commands will work against your remote server.

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
todo show <list-name>            # interactive browser (in a terminal)
todo show <list-name> --plain    # force the static rendering
todo show <list-name> --details  # static, with descriptions printed inline
todo show <list-name> -H         # full history (includes completed items)
```

In a terminal, `show` opens an **interactive browser**, which is where you both
read a list and complete items:

| key | effect |
| --- | --- |
| `↑` `↓` (or `k` `j`) | move |
| `→` (or `Enter`) | open the description · `←` folds it back |
| `x` (or `Space`) | tick the item for completion |
| `Ctrl+S` | complete every ticked item |
| `q` | quit (asks first if anything is ticked) |

Items that carry a description are marked with a `▸`, so you can tell at a
glance which ones have more to show. Ticking is **local until you confirm**: a
ticked box shows `[x]`, nothing is sent until `Ctrl+S`, and quitting with ticks
pending asks before discarding them.

Completion is *soft*: completed items are kept and flagged, not deleted, and
remain visible under `todo show <list> -H`.

The browser renders **inline**, where you typed the command: it does not clear
the screen, your terminal history stays visible above it, and the list remains
on screen after you quit — like ordinary command output that happened to be
navigable.

```
  shopping  (3)

    [ ] Buy milk
  ❯ ▾ [ ] Call the plumber
      Leak under the kitchen sink. Get a quote before
      the work starts.
    ▸ [ ] Book the tickets

  ↑/↓ move · →/⏎ open · ← close · q quit
```

When the output is **piped or redirected** the command prints a static list
instead, so `todo show shopping | grep milk` and scripts keep working:

```bash
todo show shopping --plain
# [ ] To do (3):
#
#   1.   [ ] Buy milk
#   2. ▸ [ ] Call the plumber
#   3. ▸ [ ] Book the tickets
#
#   ▸ marks an item with a description — see it with --details
```

`--details` prints the descriptions inline, which is the way to read them
without the interactive view.

By default `show` hides completed items. Add `-H` (history) to display every
item, including the ones that were marked complete. `-H` and `--details` both
use the static rendering.

### Add Items to a List

```bash
todo add <list-name> <item1> [item2] [item3...]
todo add <list-name> <item> -d "longer description"
```

**Example:**
```bash
todo add shopping "Buy cheese" "Buy yogurt"
todo add shopping "Call the plumber" -d "Leak under the kitchen sink. Get a quote first."
```

Items are appended to the end of the list. Shows the updated list after adding.

An item has a **short title** and an optional **longer description**. The title
is what every listing shows; the description is the detail you read on demand
(see [Show a List](#show-a-list)). Because a description describes one item,
`-d` only accepts a single item at a time — add them one by one, or set the
description afterwards with `todo update`.

### Update an Existing Item

```bash
todo update <list-name>
```

**Interactive command:** displays the non-completed items and asks for an
index, then opens an editor on that item:

```
  Edit item

  Title
  Appeler le plombier

  Description
  Fuite sous l'évier de la cuisine.

  À faire :
  - demander un devis
  - vérifier la garantie du joint

  tab next field · ctrl+s save · esc cancel
```

Both fields are pre-filled with their current values, so this is an edit rather
than a re-entry:

| key | effect |
| --- | --- |
| `Ctrl+S` | save and close |
| `Tab` | move between title and description |
| `Enter` | in the title: go to the description · in the description: new line |
| `Esc` | cancel, changing nothing |

Clearing the description removes it. Note that `Cmd+S` does **not** work on
macOS — a terminal never passes the Command key to the program it runs.

The description may span **several lines**, blank lines included: the browser
and `--details` both preserve the line breaks you type.

Only the fields you actually changed are sent, so editing a title never wipes a
description (and the other way round).

`todo update` needs a real terminal. In a script, add the item with
`todo add <list> "<title>" -d "<description>"` instead.

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
# (self-hosted mode: run `todo login` first)
todo create today "Review emails" "Team meeting" "Finish report"
todo show today
todo show today                  # Browse, tick with x, complete with ctrl+s
todo show today -H               # Review what was completed
```

### Shopping List
```bash
todo create shopping "Milk" "Eggs" "Bread"
todo show shopping               # Browse, tick with x, complete with ctrl+s
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

- **Local mode (default):** lists are stored in `~/.config/todolist/data.json`
  (selected by `TODO_STORAGE=json`, the default when no remote server is
  configured).
- **Self-hosted mode (optional):** your todo lists live in your server's
  **PostgreSQL** database, isolated per user. The only thing stored locally is
  `~/.config/todolist/credentials.json` (your tokens, mode `0600`). Backup /
  restore is handled by your PostgreSQL provider, not by copying a local file.

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
