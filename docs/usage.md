# Usage Guide

Complete reference for all TodoList CLI commands.

## Prerequisites

Make sure the server is running:
```bash
make dev  # Start server in background
```

## Quick Reference

| Command | Description |
|---------|-------------|
| `todo list` | List all todo lists (shows non-completed item count) |
| `todo create <name> [items...]` | Create a new todo list |
| `todo show <name>` | Display items in a list |
| `todo show <name> -H` | Show full history (all items including completed) |
| `todo add <name> <items...>` | Add new items to an existing list |
| `todo update <name>` | Edit an existing item (interactive) |
| `todo complete <name>` | Mark items as complete (interactive) |
| `todo delete <name>` | Delete an entire list |
| `todo delete-items <name> <indices...>` | Delete specific items |

## Commands

### List All Todo Lists

```bash
todo list
```

Shows all available todo lists. If no lists exist, displays "No todo lists available."

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
todo show <list-name> [--verbose]
```

**Example:**
```bash
todo show shopping
# Output:
# 1. [ ] Buy milk
# 2. [x] Buy eggs
# 3. [ ] Buy bread
```

Add `--verbose` flag to show full details (when available).

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

**Interactive command:** Displays non-completed items, prompts for index, then allows editing the item text with arrow keys (uses promptui). Press Enter to keep current value or modify and save.

### Mark Items as Complete

```bash
todo complete <list-name>
```

**Interactive command:** Displays non-completed items with indices starting at 1. Enter space-separated indices to mark as complete. Re-prompts if invalid indices are entered.

### Delete Specific Items

```bash
todo delete-items <list-name> <index1> [index2] [index3...]
```

**Example:**
```bash
todo delete-items shopping 2 4
```

Remaining items are re-indexed after deletion.

### Delete an Entire List

```bash
todo delete <list-name> [list-name2] [list-name3...]
```

**Example:**
```bash
todo delete shopping work personal
```

**Warning:** This action is permanent!

## Common Workflows

### Daily Tasks
```bash
todo create today "Review emails" "Team meeting" "Finish report"
todo show today
todo complete today 1 2
todo delete today
```

### Shopping List
```bash
todo create shopping "Milk" "Eggs" "Bread"
todo complete shopping 1 2    # Mark items as bought
todo update shopping "Coffee" # Add forgotten item
todo delete shopping          # Done shopping
```

## Tips

- Use quotes around items with spaces: `"Buy milk"` not `Buy milk`
- Keep list names lowercase and simple: `shopping`, `work-tasks`
- Use `todo list` frequently to see all your lists
- Delete completed lists regularly to stay organized

## Error Messages

**"connection refused"**: Server isn't running → `make dev`

**"todo list does not exist"**: Check spelling or create with `todo create`

**"todo list already exists"**: Use different name or delete existing list first

## Data Location

All data is stored in `~/.config/todolist/data.json`

Backup:
```bash
cp ~/.config/todolist/data.json ~/backup/todolist-$(date +%Y%m%d).json
```

## Getting Help

- Check server status: `make status`
- View logs: `tail -f /tmp/todoserver.log`
- See [installation.md](installation.md) for setup issues
- Report bugs on GitHub
