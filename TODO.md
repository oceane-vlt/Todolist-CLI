# Feature Implementation Todos

This file tracks planned features and improvements for the todolist-cli project.

> **Status (2026-09):** Items now have a title plus an optional multi-line
> description, and `todo show` is an interactive browser (Bubble Tea) where you
> read descriptions, tick items with `x` and complete them with `ctrl+s` — the
> `complete` command was folded into it and removed. Completing is therefore
> **interactive only**: there is no scripted way to complete an item any more.
>
> **Status (2026-06):** Remote storage, authentication, and deployment are done.
> The original "SQLite" plan was superseded by a remote PostgreSQL backend (Neon),
> with Supabase/JWT authentication, TLS, multi-user isolation by `user_id`, a
> gRPC-Gateway REST endpoint, and Fly.io deployment. A `todo migrate` command
> imports an existing local `data.json` into the remote store. Local JSON mode is
> kept as an optional offline/dev backend (`TODO_STORAGE=json`, the default).

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
- [x] ~~Upgrade to SQLite for more robust data handling~~ → shipped a remote **PostgreSQL** backend (Neon) instead, selectable via `TODO_STORAGE=postgres` + `DATABASE_URL` (JSON remains the default local backend)

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
- [x] Commands declare their arguments, so `--help` and usage errors show them (`todo add <list> <item> [item...]`)
- [x] A usage error exits non-zero and prints the error once (it used to print twice and exit 0)
- [x] `todo` with no argument shows the help instead of nothing

#### LIST
- [x] The number of elements displayed should be the non completed one

#### CREATE
- [x] Handle the case where user wants to create a list that already exists
- [ ] [optional] Create with *interactive* mode (add details to each item)
- [x] Display the list once created

#### SHOW
- [ ] *Show* command with no arguments should display existing todo lists and ask user to enter the list they want to view
- [x] Add a verbose option to display only the title or full details (interactive browser + `--details`)
- [x] Interactive browser: arrow-key navigation, fold/unfold descriptions, `▸` marks the items that have one
- [x] Degrade to static output when stdout is not a terminal, so pipes and scripts keep working (`--plain` forces it)
- [ ] The `-v/--verbose` flag is declared but unused — either implement it or remove it
- [x] We can search with case-insensitive (make sur there is no issue when deleting, creating, etc)
- [x] Command run with non existing list should display an error
- [x] Only show the 7 first completed items (shows first 7, use -H for all)
- [x] Add a comment/argument (--history -H) to show the full history of completed items

#### DELETE
- [x] Delete with the title of the list → if the list doesn't exist → Display the existing todo lists
- [x] [optional] Delete with a list of titles → if a list doesn't exist → Display the existing todo lists
- [ ] Delete without the title → Display the list of todo lists
- [ ] Ask for conformation before deleting

#### COMPLETE (now part of `todo show`, the `complete` command was removed)
- [x] Show the updated list once the items have been marked complete — the browser shows the ticks in place, and confirms how many were completed
- [x] If the index doesn't exist → ask again to the user — no longer applicable: you tick a row with the cursor, so an invalid index cannot be typed
- [ ] A non-interactive way to complete items (e.g. `todo show <list> --complete 2,5`) — dropped with the `complete` command; only add it back if a script needs it

#### UPDATE
- [x] Edit an item's title and description in one interactive form (Bubble Tea), replacing the promptui prompts that corrupted the display on long values
- [x] Send only the fields that actually changed, so editing a title cannot wipe a description
- [ ] Pick the item from the browser instead of typing its index

#### ADD
- [ ] If the user doesn't add the new items as arguments of the command → ask the user to add the elements they want → scan stdin → call updateItem with the scanned list
- [x] Print the list once updated
- [x] Enable create elements with description (`todo add <list> <item> -d "..."`, and `todo update` edits it)

## Testing
- [x] Add table-driven tests for JSON parsing logic
- [x] Test parseTodoListNames function with multiple scenarios
- [ ] Test gRPC methods individually with mock data
- [ ] Test CLI end-to-end with the running server (done manually against a local server; not automated)
- [x] Unit tests for the interactive views: wrapping, scrolling, marking, quit guard, and the invariant that a rendered frame never exceeds the terminal
- [ ] Add integration tests
- [ ] Validate behavior on reboot with launchd active

## Documentation
- [x] Create comprehensive README.md
- [x] Separate user documentation from development TODOs
- [x] Create organized docs/ directory structure
- [ ] Add architecture diagrams
- [ ] Add API documentation for proto file

## Future Improvements
- [x] ~~Replace JSON storage with SQLite backend~~ → done as remote **PostgreSQL** (Neon) backend
- [x] Add configuration files or environment variables (env-driven config: `TODO_STORAGE`, `DATABASE_URL`, `TODO_SERVER_ENDPOINT`, `TODO_TLS`, `SUPABASE_URL`, `SUPABASE_ANON_KEY`, etc.)
- [x] Add optional auth, TLS, or multi-user features — **done**: Supabase/JWT auth, TLS, multi-user isolation by `user_id`
- [x] Remote deployment (Fly.io) + gRPC-Gateway REST endpoint + `todo migrate` to import local `data.json`
- [ ] Support for recurring tasks
- [ ] Due dates and reminders — the storage seam already takes an `ItemUpdate` struct of optional fields, so this is a new field rather than a new signature
- [ ] Task priorities — same as above
- [ ] Tags and filtering
- [ ] Export/import functionality (CSV, JSON)
- [ ] Web interface
- [ ] Mobile app using same gRPC backend
- [ ] Linux systemd service support
- [ ] Notification feature
- [ ] Keep an historic of the completed items (and a new command to show the historic)
- [ ] macOS Quick Action/Spotlight integration (display todos in notification with keyboard shortcut)
