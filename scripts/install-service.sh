#!/bin/bash

# Script to install the todolist-server launchd service (user mode)

set -e

SERVICE_NAME="com.oceane.todolist-server"
PLIST_FILE="${SERVICE_NAME}.plist"
USER_AGENT_DIR="$HOME/Library/LaunchAgents"
TARGET_PLIST="${USER_AGENT_DIR}/${PLIST_FILE}"

echo "🚀 Installing todolist-server service (user mode, starts on login)..."

# Check that plist file exists
if [ ! -f "$PLIST_FILE" ]; then
    echo "❌ Error: $PLIST_FILE does not exist"
    echo "   Make sure to run this script from the project root"
    exit 1
fi

# Check that executable exists
if [ ! -f "$HOME/go/bin/server" ]; then
    echo "❌ Error: $HOME/go/bin/server does not exist"
    echo "   Run first: make install"
    exit 1
fi

# Create directories if needed
mkdir -p "$HOME/.config/todolist"
mkdir -p "$USER_AGENT_DIR"

# Copy plist file
echo "📋 Copying plist file to $USER_AGENT_DIR..."
cp "$PLIST_FILE" "$TARGET_PLIST"
chmod 644 "$TARGET_PLIST"

# Load the service
echo "⚡ Loading service..."
launchctl load "$TARGET_PLIST"

# Wait a moment for the service to start
sleep 1

# Verify service is loaded
if launchctl list | grep -q "$SERVICE_NAME"; then
    echo "✅ Service installed and started successfully!"
    echo ""
    echo "📊 Installation details:"
    echo "   Mode:                User (starts on login, no sudo required)"
    echo "   Location:            $TARGET_PLIST"
    echo ""
    echo "📊 Useful commands:"
    echo "   View logs:           tail -f ~/.config/todolist/server.log"
    echo "   View errors:         tail -f ~/.config/todolist/server-error.log"
    echo "   Check status:        launchctl list | grep todolist"
    echo "   Stop service:        launchctl unload $TARGET_PLIST"
    echo "   Start service:       launchctl load $TARGET_PLIST"
    echo "   Uninstall:           make uninstall-service"
else
    echo "❌ Error: Service could not be loaded"
    echo "   Check logs: tail ~/.config/todolist/server-error.log"
    exit 1
fi
