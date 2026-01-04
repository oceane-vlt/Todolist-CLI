#!/bin/bash

# Script to uninstall the todolist-server launchd service (user mode)

set -e

SERVICE_NAME="com.oceane.todolist-server"
PLIST_FILE="${SERVICE_NAME}.plist"
USER_AGENT_DIR="$HOME/Library/LaunchAgents"
TARGET_PLIST="${USER_AGENT_DIR}/${PLIST_FILE}"

echo "🗑️  Uninstalling todolist-server service..."

# Check if service is loaded
if launchctl list | grep -q "$SERVICE_NAME"; then
    echo "⏹️  Stopping service..."
    launchctl unload "$TARGET_PLIST" 2>/dev/null || true
fi

# Remove plist file
if [ -f "$TARGET_PLIST" ]; then
    echo "📋 Removing plist file..."
    rm "$TARGET_PLIST"
fi

echo "✅ Service uninstalled successfully!"
echo ""
echo "Note: Log files are still present in ~/.config/todolist/"
echo "      Remove them manually if needed: rm ~/.config/todolist/server*.log"
