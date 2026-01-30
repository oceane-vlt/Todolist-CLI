#!/bin/bash

# Script to install the todolist-server launchd service (user mode)

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

SERVICE_NAME="com.todolist.server"
TEMPLATE_FILE="${SERVICE_NAME}.plist.template"
PLIST_FILE="${SERVICE_NAME}.plist"
USER_AGENT_DIR="$HOME/Library/LaunchAgents"
TARGET_PLIST="${USER_AGENT_DIR}/${PLIST_FILE}"

echo -e "${BLUE}🚀 Installing todolist-server service (user mode, starts on login)...${NC}\n"

# Step 1: Detect GOBIN
GOBIN=${GOBIN:-$(go env GOBIN)}
if [ -z "$GOBIN" ]; then
    GOBIN=$(go env GOPATH)/bin
fi

if [ -z "$GOBIN" ]; then
    echo -e "${RED}❌ Error: Could not detect GOBIN. Please set GOBIN or GOPATH.${NC}"
    exit 1
fi

echo -e "Detected GOBIN: ${GREEN}$GOBIN${NC}"

# Step 2: Check that server binary exists
if [ ! -f "$GOBIN/server" ]; then
    echo -e "${RED}❌ Error: $GOBIN/server does not exist${NC}"
    echo "   Run first: make install"
    exit 1
fi

echo -e "${GREEN}✓ Server binary found${NC}"

# Step 3: Generate plist from template
if [ ! -f "$TEMPLATE_FILE" ]; then
    echo -e "${RED}❌ Error: $TEMPLATE_FILE does not exist${NC}"
    echo "   Make sure to run this script from the project root"
    exit 1
fi

echo -e "\n${BLUE}📋 Generating $PLIST_FILE from template...${NC}"

# Replace placeholders
sed -e "s|{{GOBIN}}|$GOBIN|g" \
    -e "s|{{HOME}}|$HOME|g" \
    "$TEMPLATE_FILE" > "$PLIST_FILE"

echo -e "${GREEN}✓ Generated $PLIST_FILE${NC}"

# Step 4: Create directories if needed
mkdir -p "$HOME/.config/todolist"
mkdir -p "$USER_AGENT_DIR"

# Step 5: Copy plist file
echo -e "\n${BLUE}📦 Installing plist to $USER_AGENT_DIR...${NC}"
cp "$PLIST_FILE" "$TARGET_PLIST"
chmod 644 "$TARGET_PLIST"

echo -e "${GREEN}✓ Copied to $USER_AGENT_DIR${NC}"

# Step 6: Unload existing service (if any)
if launchctl list | grep -q "$SERVICE_NAME"; then
    echo -e "\n${YELLOW}⚠️  Service already loaded, reloading...${NC}"
    launchctl unload "$TARGET_PLIST" 2>/dev/null || true
fi

# Step 7: Load the service
echo -e "\n${BLUE}⚡ Loading service...${NC}"
launchctl load "$TARGET_PLIST"

# Wait a moment for the service to start
sleep 1

# Step 8: Verify service is loaded
if launchctl list | grep -q "$SERVICE_NAME"; then
    echo -e "\n${GREEN}✅ Service installed and started successfully!${NC}\n"
    echo -e "${BLUE}📊 Installation details:${NC}"
    echo -e "   Mode:                User (starts on login, no sudo required)"
    echo -e "   Location:            $TARGET_PLIST"
    echo -e "   Binary:              $GOBIN/server"
    echo ""
    echo -e "${BLUE}📊 Useful commands:${NC}"
    echo -e "   View logs:           ${GREEN}tail -f ~/.config/todolist/server.log${NC}"
    echo -e "   View errors:         ${GREEN}tail -f ~/.config/todolist/server-error.log${NC}"
    echo -e "   Check status:        ${GREEN}launchctl list | grep todolist${NC}"
    echo -e "   Stop service:        ${GREEN}launchctl unload $TARGET_PLIST${NC}"
    echo -e "   Start service:       ${GREEN}launchctl load $TARGET_PLIST${NC}"
    echo -e "   Uninstall:           ${GREEN}make uninstall-service${NC}"
else
    echo -e "${RED}❌ Error: Service could not be loaded${NC}"
    echo "   Check logs: tail ~/.config/todolist/server-error.log"
    exit 1
fi
