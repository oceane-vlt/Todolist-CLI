# Installation Guide

This guide covers how to install and set up the TodoList CLI on your system.

## Prerequisites

- **Go 1.24 or later** - [Download Go](https://golang.org/dl/)
- **Supported platforms**: macOS, Linux, Windows (launchd daemon is macOS-only)

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

## Installation

Install using `go install`:

```bash
# Install the CLI and server
go install github.com/oceane-vlt/todolist-cli/cmd/todo@latest
go install github.com/oceane-vlt/todolist-cli/cmd/server@latest
```

This installs two binaries to `~/go/bin/`:
- `todo` - The CLI client
- `server` - The gRPC server

Verify installation:
```bash
which todo      # macOS/Linux: Should show path to todo
which server    # macOS/Linux: Should show path to server
```

## Choose Your Server Mode

You have **two options** for running the server:

---

## Option 1: Manual Server Mode (Recommended for Getting Started)

This mode lets you start and stop the server manually. Best for:
- Quick testing and development
- Understanding how the system works
- Situations where you don't need the server running all the time

### Starting the Server Manually

**Quick method** (recommended):
```bash
make dev
```

This will:
1. Stop any running server instances
2. Rebuild and reinstall the binaries
3. Start the server in the background
4. Log output to `/tmp/todoserver.log`

### Checking Server Status

```bash
# Using make
make status

# Or manually
pgrep -f server
```

### Viewing Logs

```bash
# Real-time logs
tail -f /tmp/todoserver.log

# Or with make
make logs
```

### Stopping the Server

```bash
# Using make
make stop

# Or manually
pkill -f server
```

### Restarting After Code Changes

```bash
make dev  # Rebuilds, reinstalls, and restarts
```

---

## Option 2: Automatic Server with Daemon (macOS Only)

**Recommended for daily use.** The server runs in the background and starts automatically on login.

```bash
make install-service
```

The script automatically:
- Detects your `$GOBIN` and `$HOME` paths
- Generates the launchd plist file
- Installs and starts the service

See [daemon-setup.md](daemon-setup.md) for details and troubleshooting.

---

## Verifying Installation

Once the server is running (via either method), test the CLI:

```bash
# List all todo lists (should be empty initially)
todo list

# Create your first list
todo create test "First item" "Second item"

# Show the list
todo show test

# Success! Your installation is working.
```

## Data Storage Location

All todo data is stored in:
```
~/.config/todolist/data.json
```

This directory is created automatically on first use.

## Troubleshooting

### "command not found: todo"

Your `~/go/bin` is not in your PATH. Add it:
```bash
export PATH="$HOME/go/bin:$PATH"
```

Make it permanent by adding the line to `~/.zshrc` or `~/.bashrc`.

### "connection refused" or "unavailable"

The server isn't running. Start it:
```bash
make dev
```

Check if it's running:
```bash
make status
```

### Server starts but crashes immediately

Check the logs:
```bash
tail -100 /tmp/todoserver.log
```

Common issues:
- **Port 50051 already in use**: Another instance is running. Stop it with `make stop`
- **Permission denied on data file**: Check permissions on `~/.config/todolist/`

### Can't find server binary

Reinstall:
```bash
make clean
make install
```

## Uninstallation

To remove the todolist CLI:

```bash
# Stop the server
make stop

# Uninstall binaries
make uninstall

# Remove data (optional)
rm -rf ~/.config/todolist/
```

## Next Steps

- Read the [Usage Guide](usage.md) to learn all available commands
- Check [TODO.md](../TODO.md) for planned features

## Getting Help

If you encounter issues:
1. Check the logs: `tail -f /tmp/todoserver.log`
2. Verify the server is running: `make status`
3. Try rebuilding: `make clean && make install && make dev`
4. Open an issue on GitHub with log output and error messages
