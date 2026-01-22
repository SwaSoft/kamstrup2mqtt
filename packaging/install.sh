#!/bin/bash

# Exit on error
set -e

# Default paths
BIN_DEST="/usr/local/bin/kamstrup2mqtt"
CONFIG_DIR="/etc/kamstrup2mqtt"
CONFIG_FILE="$CONFIG_DIR/config.json"
SERVICE_FILE="/etc/systemd/system/kamstrup2mqtt.service"
SERVICE_NAME="kamstrup2mqtt"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting kamstrup2mqtt installation...${NC}"

# Check for root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}This script must be run as root (or with sudo)${NC}"
   exit 1
fi

# Determine source directory
SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 1. Dependency Check
if ! command -v jq &> /dev/null; then
    echo -e "${YELLOW}Warning: 'jq' not found. Configuration merging will be disabled.${NC}"
    echo -e "Installing 'jq' is highly recommended for upgrades."
    HAS_JQ=false
else
    HAS_JQ=true
fi

# 2. Binary Installation
echo -e "Installing binary to $BIN_DEST..."
# Detect architecture and copy the right binary if multiple exist
if [[ -f "$SOURCE_DIR/kamstrup2mqtt" ]]; then
    cp "$SOURCE_DIR/kamstrup2mqtt" "$BIN_DEST"
elif [[ "$(uname -m)" == "x86_64" ]] && [[ -f "$SOURCE_DIR/kamstrup2mqtt-amd64" ]]; then
    cp "$SOURCE_DIR/kamstrup2mqtt-amd64" "$BIN_DEST"
elif [[ "$(uname -m)" == "aarch64" ]] && [[ -f "$SOURCE_DIR/kamstrup2mqtt-arm64" ]]; then
    cp "$SOURCE_DIR/kamstrup2mqtt-arm64" "$BIN_DEST"
else
    echo -e "${RED}Error: kamstrup2mqtt binary not found in $SOURCE_DIR${NC}"
    exit 1
fi
chmod +x "$BIN_DEST"

# 3. User Creation
if ! id "$SERVICE_NAME" &>/dev/null; then
    echo -e "Creating system user $SERVICE_NAME..."
    useradd -r -s /usr/sbin/nologin -G dialout "$SERVICE_NAME"
fi

# 4. Config Management
mkdir -p "$CONFIG_DIR"
if [[ ! -f "$CONFIG_FILE" ]]; then
    echo -e "Creating initial configuration at $CONFIG_FILE..."
    cp "$SOURCE_DIR/config.json.example" "$CONFIG_FILE"
    chown "$SERVICE_NAME:$SERVICE_NAME" "$CONFIG_FILE"
    chmod 640 "$CONFIG_FILE"
else
    echo -e "Configuration exists at $CONFIG_FILE. Checking for updates..."
    if [[ "$HAS_JQ" == "true" ]]; then
        # Use jq to merge new keys from example but keep existing values
        # This is a deep merge that preserves existing values
        TMP_CONFIG=$(mktemp)
        jq -s '.[0] * .[1]' "$SOURCE_DIR/config.json.example" "$CONFIG_FILE" > "$TMP_CONFIG"
        if ! diff -q "$CONFIG_FILE" "$TMP_CONFIG" > /dev/null; then
            echo -e "${YELLOW}Merging new configuration options...${NC}"
            cp "$TMP_CONFIG" "$CONFIG_FILE"
            chown "$SERVICE_NAME:$SERVICE_NAME" "$CONFIG_FILE"
            chmod 640 "$CONFIG_FILE"
        else
            echo -e "Configuration is up to date."
        fi
        rm "$TMP_CONFIG"
    else
        echo -e "${YELLOW}Skipping configuration merge because 'jq' is missing.${NC}"
    fi
fi

# 5. Service Installation
echo -e "Installing systemd service..."
cp "$SOURCE_DIR/kamstrup2mqtt.service" "$SERVICE_FILE"
systemctl daemon-reload

# 6. Interactive Setup (Optional)
read -p "Do you want to enable and start the service now? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    systemctl enable "$SERVICE_NAME"
    systemctl restart "$SERVICE_NAME"
    echo -e "${GREEN}Service started and enabled.${NC}"
else
    echo -e "Service installed but not started. Run 'systemctl start $SERVICE_NAME' to start."
fi

echo -e "${GREEN}Installation complete!${NC}"
echo -e "Check logs with: journalctl -u $SERVICE_NAME -f"
