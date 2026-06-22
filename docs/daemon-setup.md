# Daemon Setup (macOS)

> **Note (remote storage migration):** with the target architecture the server
> is hosted remotely (Fly.io) and the CLI no longer runs a local server — see
> [`deployment.md`](./deployment.md). This launchd guide remains valid for the
> **local JSON** mode (`TODO_STORAGE` unset/`json`), which still works.

Guide for running the TodoList server as a launchd service that starts automatically.

## What is a launchd service?

On macOS, **launchd** manages background processes. When installed as a launchd service:

- ✅ Server starts automatically on login
- ✅ Server restarts automatically if it crashes
- ✅ Logs saved to `~/.config/todolist/server.log`
- ✅ No need to manually start the server

## Quick Setup

### Install the Service

```bash
make install-service
```

The installation script automatically:
1. Detects your `$GOBIN` and `$HOME` paths
2. Generates `com.todolist.server.plist` from template
3. Installs to `~/Library/LaunchAgents/`
4. Loads and starts the service

No manual configuration needed!

### Check Status

```bash
make service-status
```

Or manually:
```bash
launchctl list | grep com.todolist.server
```

### View Logs

Real-time logs:
```bash
make service-logs
```

Or directly:
```bash
tail -f ~/.config/todolist/server.log
tail -f ~/.config/todolist/server-error.log
```

### Uninstall the Service

```bash
make uninstall-service
```

This stops and removes the service. Log files remain in `~/.config/todolist/`.

## Service Configuration

The plist file configures the service:

| Setting | Value | Description |
|---------|-------|-------------|
| Label | `com.todolist.server` | Unique service identifier |
| RunAtLoad | `true` | Start on login |
| KeepAlive | `true` | Restart on crash |
| ThrottleInterval | `10` | Wait 10s before restart |

## Troubleshooting

### Service won't start

Check error log:
```bash
cat ~/.config/todolist/server-error.log
```

Try running server manually:
```bash
~/go/bin/server
```

### Service crashes immediately

Common issues:
- Port 50051 already in use (another server running)
- Incorrect paths in plist file
- Missing data directory

### Update server binary

After code changes:
```bash
make install            # Rebuild and install
launchctl stop com.todolist.server
launchctl start com.todolist.server
```

## Development vs Daemon

### Development Mode (`make dev`)
- Manual start/stop
- Logs to `/tmp/todoserver.log`
- Easy to restart for testing
- **Recommended for development**

### Daemon Mode (`make install-service`)
- Automatic start on login
- Auto-restart on crash
- Logs to `~/.config/todolist/`
- **Recommended for daily use**

## File Locations

| File | Location |
|------|----------|
| Service definition | `~/Library/LaunchAgents/com.todolist.server.plist` |
| Server binary | `~/go/bin/server` |
| Data file | `~/.config/todolist/data.json` |
| Stdout logs | `~/.config/todolist/server.log` |
| Error logs | `~/.config/todolist/server-error.log` |

## Getting Help

The daemon setup is fully automated. If you encounter issues, check the logs or file a GitHub issue.
