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
- [x] Initialize the Go module and project structure.
- [x] Create folders for server, client CLI, proto files, storage, and business logic.
- [ ] Validate project structure.

## gRPC API Design
- [x] Define basic proto file for firsts CRUD operations (lists).
- [x] Generate the gRPC server and client code.
- [x] Define GetTodoLists RPC method to fetch list names.
- [ ] Validate all request/response messages and service layout.

## Data Storage (JSON first)
- [x] Create JSON parsing functions to read todo data.
- [x] Implement map-based structure for nested lists (map[string][]TodoItem).
- [x] Add GetTodoListsTitles function to extract list names from JSON.
- [ ] Implement full CRUD operations (create, update, delete).
- [ ] Add ID generation and validation logic.

## Business Logic
- [x] Implement GetTodoLists logic to return list names.
- [x] Connect parsing logic with gRPC service handlers.
- [ ] Implement full CRUD logic for lists and items.
- [x] Easy implementation for **create** CRUD
    - [x] Enable Create New list with elements
        - [ ] Create with *interactive* mode (add details in each item)
        - [ ] Display the list once created
    - [x] Implement *Read* method
    - [ ] Implement *Delete* method
        - [ ] Delete with the title of the list -> if the list don't exist -> Display the existing todo lists
        - [ ] [optional] Delete with a list of title if -> if a list doesn't exist -> Display the existing todo lists
        - [ ] Delete without the title -> Display the list of todo lists
    - [ ] Implement *Update* method
    - [x] Make sure I handle the case user want to create a list that already exist
    - [x] Remplace deprecated `ioutil.WriteFile` and `ioutil.ReadFile`
- [ ] Add business rules and validation.

## gRPC Server
- [x] Create the server executable (cmd/server).
- [x] Configure the server to listen on 127.0.0.1:50051.
- [x] Implement GetTodoLists RPC handler.
- [x] Test the server manually by running it in a terminal.
- [ ] Add server configuration (storage path, logging).
- [ ] Implement remaining RPC methods (Create, Update, Delete).

## launchd Integration
- [ ] Create a launchd plist file to run the server at system startup.
- [ ] Configure WorkingDirectory, ProgramArguments, and KeepAlive.
- [ ] Load the plist with launchctl so the server runs continuously in the background.
- [ ] Verify the server restarts automatically at reboot.

## CLI Client
- [x] Create the CLI structure with Cobra framework.
- [x] Implement global gRPC client connection.
- [x] Add 'list' command to fetch and display todo lists.
- [ ] Implement remaining commands (create, update, delete, show).
- [ ] Add formatting, error messages, and help text.
- [ ] Improve CLI ergonomics and user experience.

## Testing
- [x] Add table-driven tests for JSON parsing logic.
- [x] Test parseTodoListNames function with multiple scenarios.
- [ ] Test gRPC methods individually with mock data.
- [ ] Test CLI end-to-end with the running server.
- [ ] Add integration tests.
- [ ] Validate behavior on reboot with launchd active.

## Future Improvements
- [ ] Replace JSON storage with SQLite backend.
- [ ] Add configuration files or environment variables.
- [ ] Add optional auth, TLS, or multi-user features.
