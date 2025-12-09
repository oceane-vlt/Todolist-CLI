# TodoList Project — gRPC Client/Server Architecture with launchd

This project aims to build a fully structured **TodoList application** using a modern **client–server architecture** based on **gRPC**.
The goal is to create a clean separation between:

* a **CLI client**, which the user interacts with, and
* a **gRPC server**, which manages all todo lists, applies business logic, and handles data storage.

The server runs continuously in the background (managed by `launchd` on macOS), while the CLI communicates with it through gRPC calls to perform operations such as creating, listing, updating, and deleting todo items or lists.

In the first version, todo lists are stored locally in a **JSON file**, allowing for a simple and lightweight persistence layer.
A future version will upgrade the storage to **SQLite** for more robust data handling and scalability.

This project is designed to explore and learn:

* gRPC service design and communication flow
* clean client–server architecture in Go
* data persistence strategies
* background services via launchd
* CLI ergonomics and practical developer tooling

The final result is a modular, extensible, and educational codebase demonstrating how to structure a real-world Go application using gRPC.

---
# Feature implementation Todos
## Project Setup
- [ ] Initialize the Go module and project structure.
- [ ] Create folders for server, client CLI, proto files, storage, and business logic.

## gRPC API Design
- [ ] Define the proto file for CRUD operations (lists and items).
- [ ] Generate the gRPC server and client code.
- [ ] Validate request/response messages and service layout.

## Data Storage (JSON first)
- [ ] Create a storage interface for todo list operations.
- [ ] Implement JSON-based persistence (load/save to file).
- [ ] Add basic validation and ID handling.

## Business Logic
- [ ] Implement CRUD logic for lists and items.
- [ ] Connect business logic with the JSON storage implementation.
- [ ] Integrate business logic into gRPC service handlers.

## gRPC Server
- [ ] Create the server executable (cmd/server).
- [ ] Configure the server to listen on localhost (e.g., 127.0.0.1:50051).
- [ ] Add server configuration (port, storage path).
- [ ] Test the server manually by running it in a terminal.

## launchd Integration
- [ ] Create a launchd plist file to run the server at system startup.
- [ ] Configure WorkingDirectory, ProgramArguments, and KeepAlive.
- [ ] Load the plist with launchctl so the server runs continuously in the background.
- [ ] Verify the server restarts automatically at reboot.

## CLI Client
- [ ] Create the CLI structure (commands + subcommands).
- [ ] Implement each command to call the gRPC server.
- [ ] Add formatting, error messages, and help text.

## Testing
- [ ] Test JSON storage directly.
- [ ] Test gRPC methods individually.
- [ ] Test CLI end-to-end with the running server.
- [ ] Validate behavior on reboot with launchd active.

## Future Improvements
- [ ] Replace JSON storage with SQLite backend.
- [ ] Add configuration files or environment variables.
- [ ] Add optional auth, TLS, or multi-user features.
