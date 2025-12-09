# Notification Feature

This feature triggers a macOS notification every weekday at **10:30 AM (Europe/Paris time)**.
A Go program generates a message, an AppleScript captures that message and displays it, and a `launchd` job schedules the execution automatically.

---
# Feature implementation Todos
## Go Program
- [x] Create the function that generates the message.
- [x] Print the message in main() using fmt.Println.
    - [x] Only for elements in the same list
    - [x] For elements from differents lists
- [x] Build and install the binary (go install).

## AppleScript
- [ ] Write the AppleScript that executes the Go program.
- [ ] Capture the program’s output.
- [ ] Display a macOS notification with the output.

## launchd
- [ ] Create the .plist file for periodic execution.
- [ ] Configure the execution interval (StartInterval).
- [ ] Load the service using launchctl.

## Testing
- [ ] Test the AppleScript manually.
- [ ] Verify that launchd triggers the notification automatically.

## Makefile (optional)
- [ ] Add a build command.
- [ ] Add an install command.
- [ ] Add a reload command.
- [ ] Add an all command (build + install + reload).
