# Daemon Setup (macOS)

Advanced guide for running the TodoList server as a launchd service that starts automatically.

⚠️ **Current Status**: This setup requires manual path configuration and is not yet ready for general use.

## What is a launchd service?

On macOS, **launchd** manages background processes. When installed as a launchd service:

- ✅ Server starts automatically on login
- ✅ Server restarts automatically if it crashes
- ✅ Logs saved to `~/.config/todolist/server.log`
- ✅ No need to manually start the server

## Current Limitations

The plist configuration file contains hardcoded paths specific to the original developer's machine:
- `/Users/oceane.valat/go/bin/server`
- `/Users/oceane.valat/.config/todolist`

**TODO**: These need to be templated with variables like `{{HOME}}` and `{{GOBIN}}` that can be replaced during installation.

## Manual Setup (Advanced Users Only)

If you're comfortable editing configuration files, you can set this up manually.

### Prerequisites

1. Install binaries:
   ```bash
   make install
   ```

2. Verify server binary exists:
   ```bash
   ls -l ~/go/bin/server
   ```

### Edit the Plist File

Edit [com.oceane.todolist-server.plist](../com.oceane.todolist-server.plist):

```xml
<!-- Line 12: Update to your path -->
<string>/Users/YOUR_USERNAME/go/bin/server</string>

<!-- Line 17: Update to your path -->
<string>/Users/YOUR_USERNAME/.config/todolist</string>

<!-- Line 29: Update to your path -->
<string>/Users/YOUR_USERNAME/.config/todolist/server.log</string>

<!-- Line 33: Update to your path -->
<string>/Users/YOUR_USERNAME/.config/todolist/server-error.log</string>

<!-- Line 39: Update to your path -->
<string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/Users/YOUR_USERNAME/go/bin</string>
```

Replace `YOUR_USERNAME` with your actual username (`echo $USER`).

### Install the Service

```bash
make install-service
```

This will:
1. Copy the plist to `~/Library/LaunchAgents/`
2. Load and start the service
3. Configure automatic restart on crash

### Check Status

```bash
make service-status
```

Or manually:
```bash
launchctl list | grep com.oceane.todolist-server
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
| Label | `com.oceane.todolist-server` | Unique service identifier |
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
launchctl stop com.oceane.todolist-server
launchctl start com.oceane.todolist-server
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
- **Would be recommended for daily use** (once templating is implemented)

## Future Plans

To make this production-ready, we need to:

1. Create `com.oceane.todolist-server.plist.template` with placeholders:
   ```xml
   <string>{{HOME}}/go/bin/server</string>
   <string>{{HOME}}/.config/todolist</string>
   ```

2. Create `scripts/generate-plist.sh` to replace variables:
   ```bash
   sed "s|{{HOME}}|$HOME|g" template > com.oceane.todolist-server.plist
   ```

3. Update `install-service.sh` to generate plist first

4. Add `*.plist` to `.gitignore` (keep only template)

See [TODO.md](../TODO.md) for tracking this work.

## File Locations

| File | Location |
|------|----------|
| Service definition | `~/Library/LaunchAgents/com.oceane.todolist-server.plist` |
| Server binary | `~/go/bin/server` |
| Data file | `~/.config/todolist/data.json` |
| Stdout logs | `~/.config/todolist/server.log` |
| Error logs | `~/.config/todolist/server-error.log` |

## Getting Help

For now, **use manual server mode** ([installation.md](installation.md)) until daemon setup is properly templated.

Report issues on GitHub.
