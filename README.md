# TodoList CLI

A modern **command-line todo list manager** built with **Go** and **gRPC**, featuring a client-server architecture for efficient data management.

This project serves as a practical learning project for understanding how gRPC works in a real-world application. While a simpler approach could have been used for a local CLI tool, gRPC provides valuable experience with modern service communication patterns.

## Features

- 🚀 Fast gRPC-based client-server architecture
- 📝 Create and manage multiple todo lists
- ✅ Mark items as complete
- 🔄 Update and delete lists or items
- 💾 Local JSON storage (SQLite planned)
- 🔧 Optional background server via launchd (macOS)

## Quick Start

### Installation

**Platforms**: macOS, Linux, Windows

```bash
# Install the CLI and server
go install github.com/oceane-vlt/todolist-cli/cmd/todo@latest
go install github.com/oceane-vlt/todolist-cli/cmd/server@latest
```

This installs two binaries to `~/go/bin/` (or `%USERPROFILE%\go\bin` on Windows):
- `todo` - The CLI client
- `server` - The gRPC server

Make sure `~/go/bin` is in your PATH.

### Running the Server

You have two options:

#### Option 1: Manual Server (Quick Start)

```bash
# Start server in background
make dev

# Use the CLI
todo list
todo create mylist
todo show mylist

# Stop server when done
make stop
```

#### Option 2: Automatic Server with Daemon (Recommended for Daily Use)

```bash
make install-service
```

The server will:
- Start automatically on login
- Restart on crash
- Run in the background

The installation script automatically detects your `$GOBIN` and `$HOME` paths and generates the plist file for you. See [docs/daemon-setup.md](docs/daemon-setup.md) for details.

### Basic Usage Example

```bash
# List all todo lists
todo list

# Create a new list with items
todo create shopping "Buy milk" "Buy eggs" "Buy bread"

# Show a specific list
todo show shopping

# Add items to an existing list
todo add shopping "Buy cheese" "Buy milk"

# Mark items as complete (interactive)
todo complete shopping

# Delete an entire list
todo delete shopping
```

See [docs/usage.md](docs/usage.md) for complete CLI reference.

## Documentation

- [Installation Guide](docs/installation.md) - Detailed installation instructions
- [Usage Guide](docs/usage.md) - Complete CLI command reference
- [Daemon Setup](docs/daemon-setup.md) - Configure automatic server startup (macOS)

## Project Structure

```
todolist-cli/
├── cmd/
│   ├── todo/          # CLI client commands
│   └── server/        # gRPC server
├── proto/             # Protocol Buffer definitions
├── libs/
│   └── storage/       # Data persistence layer
├── scripts/           # Installation and service scripts
└── docs/              # Documentation
```

## Architecture

This project uses a **client-server architecture**:

- **CLI Client**: Lightweight command-line interface that communicates with the server
- **gRPC Server**: Handles business logic, data validation, and storage
- **Protocol Buffers**: Defines the contract between client and server

The server runs continuously in the background, while the CLI makes quick RPC calls to perform operations. This design allows for future enhancements like web interfaces or mobile clients sharing the same backend.

Data is stored in `~/.config/todolist/data.json`.

## Requirements

- Go 1.24 or later
- macOS, Linux, or Windows
- Note: Daemon service (automatic server startup) is macOS-only via launchd

## License

MIT License - see LICENSE file for details.
