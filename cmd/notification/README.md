# Notification Feature (Future)

🚧 **Status**: Planned feature, no release date set

## Overview

A notification system that would display todo reminders via macOS notifications at scheduled times.

## Planned Implementation

The feature would work by:
1. **Go program** generates notification messages from your todo lists
2. **AppleScript** captures the output and displays macOS notifications
3. **launchd** schedules automatic execution (e.g., daily at 10:30 AM)
