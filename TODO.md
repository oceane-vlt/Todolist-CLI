# Feature Implementation Todos

This file tracks planned features and improvements for the todolist-cli project.

## Project Setup
- [x] Initialize the Go module and project structure
- [x] Create folders for server, client CLI, proto files, storage, and business logic
- [ ] Validate project structure

## gRPC API Design
- [x] Define basic proto file for first CRUD operations (lists)
- [x] Generate the gRPC server and client code
- [x] Define GetTodoLists RPC method to fetch list names
- [ ] Validate all request/response messages and service layout

## Data Storage (JSON first)
- [x] Create JSON parsing functions to read todo data
- [x] Implement map-based structure for nested lists (map[string][]TodoItem)
- [ ] Upgrade to SQLite for more robust data handling

## Business Logic
- [x] Implement GetTodoLists logic to return list names
- [x] Connect parsing logic with gRPC service handlers
- [x] **Easy** implementation for all CRUD
    - [x] Implement *Create* method
        - [x] Enable Create New list with elements
    - [x] Implement *Read* method
        - [x] Return the existing todo lists
        - [x] Return the items in a certain todo list
    - [x] Implement method *Update* to update an item in a todo list
    - [x] Implement *Delete* method
        - [x] Delete an entire list
        - [x] Delete items in a list
    - [x] Implement *Add* method (add items in a list)
- [x] Replace deprecated `ioutil.WriteFile` and `ioutil.ReadFile`
- [ ] Add comprehensive business rules and validation
- [ ] Uniformize error handling across all storage functions
    - [x] Fix showData.go bug (returns nil, nil instead of error)
    - [x] Update deleteItemsData.go to use displayList() helper
    - [x] Update updateData.go to use displayList() helper

## gRPC Server
- [x] Create the server executable (cmd/server)
- [x] Configure the server to listen on 127.0.0.1:50051
- [x] Implement GetTodoLists RPC handler
- [x] Test the server manually by running it in a terminal
- [ ] Add server configuration (storage path, logging)
- [x] Implement remaining RPC methods (Create, Update, Delete)

## launchd Integration (macOS)
- [x] Create a launchd plist file to run the server at startup
- [x] Configure WorkingDirectory, ProgramArguments, and KeepAlive
- [x] Create install/uninstall scripts for user-mode service
- [x] Load the plist with launchctl so the server runs continuously
- [x] Verify the server restarts automatically on crash
- [x] **Template the plist file for portability**
    - [x] Create com.todolist.server.plist.template with {{HOME}} and {{GOBIN}} placeholders
    - [x] Update install-service.sh to generate plist before installing
    - [x] Add *.plist to .gitignore (keep only template in repo)
- [ ] Verify the server restarts automatically at reboot

## CLI Client
- [x] Create the CLI structure with Cobra framework
- [x] Implement global gRPC client connection
- [x] Add 'list' command to fetch and display todo lists
- [ ] Implement remaining commands (create, update, delete, show)
- [ ] Improve CLI ergonomics and user experience

### CLI Command Improvements

### General
- [x] Improve overall display (colors, emoji, etc)
- [x] Improve errors message for all CRUD methods
- [ ] Having suggestion when we start typing commands

#### LIST
- [x] The number of elements displayed should be the non completed one

#### CREATE
- [x] Handle the case where user wants to create a list that already exists
- [ ] [optional] Create with *interactive* mode (add details to each item)
- [x] Display the list once created

#### SHOW
- [ ] *Show* command with no arguments should display existing todo lists and ask user to enter the list they want to view
- [x] Add a verbose option to display only the title or full details
- [x] We can search with case-insensitive (make sur there is no issue when deleting, creating, etc)
- [x] Command run with non existing list should display an error
- [x] Only show the 7 first completed items (shows first 7, use -H for all)
- [x] Add a comment/argument (--history -H) to show the full history of completed items

#### DELETE
- [x] Delete with the title of the list → if the list doesn't exist → Display the existing todo lists
- [x] [optional] Delete with a list of titles → if a list doesn't exist → Display the existing todo lists
- [ ] Delete without the title → Display the list of todo lists
- [ ] Ask for conformation before deleting

#### COMPLETE
- [ ] Show the updated list once the items have been marked complete 
- [x] If the index doesn't exist → ask again to the user

#### ADD
- [ ] If the user doesn't add the new items as arguments of the command → ask the user to add the elements they want → scan stdin → call updateItem with the scanned list
- [x] Print the list once updated
- [ ] Enable create elements with description

## Testing
- [x] Add table-driven tests for JSON parsing logic
- [x] Test parseTodoListNames function with multiple scenarios
- [ ] Test gRPC methods individually with mock data
- [ ] Test CLI end-to-end with the running server
- [ ] Add integration tests
- [ ] Validate behavior on reboot with launchd active

## Documentation
- [x] Create comprehensive README.md
- [x] Separate user documentation from development TODOs
- [x] Create organized docs/ directory structure
- [ ] Add architecture diagrams
- [ ] Add API documentation for proto file

## Future Improvements
- [ ] Replace JSON storage with SQLite backend
- [ ] Add configuration files or environment variables
- [ ] Add optional auth, TLS, or multi-user features
- [ ] Support for recurring tasks
- [ ] Due dates and reminders
- [ ] Task priorities
- [ ] Tags and filtering
- [ ] Export/import functionality (CSV, JSON)
- [ ] Web interface
- [ ] Mobile app using same gRPC backend
- [ ] Linux systemd service support
- [ ] Notification feature
- [ ] Keep an historic of the completed items (and a new command to show the historic)
- [ ] macOS Quick Action/Spotlight integration (display todos in notification with keyboard shortcut)
