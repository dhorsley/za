#!/bin/bash

# Za Syntax Highlighting Installation Script for Neovim
# This script installs the Lua-based za syntax highlighting

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Installing Za Syntax Highlighting for Neovim${NC}"

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ZADIR="$SCRIPT_DIR"

# Check if Neovim config directory exists
NVIM_CONFIG="$HOME/.config/nvim"
if [ ! -d "$NVIM_CONFIG" ]; then
    echo -e "${YELLOW}Creating Neovim config directory...${NC}"
    mkdir -p "$NVIM_CONFIG"
fi

# Create lua/za directory structure
echo -e "${YELLOW}Creating lua directory structure...${NC}"
mkdir -p "$NVIM_CONFIG/lua/za"

# Copy Lua files
echo -e "${YELLOW}Copying za module files...${NC}"
cp "$ZADIR/lua/za-nvim/init.lua" "$NVIM_CONFIG/lua/za/"
cp "$ZADIR/lua/za-nvim/filetype.lua" "$NVIM_CONFIG/lua/za/"
cp "$ZADIR/lua/za-nvim/syntax.lua" "$NVIM_CONFIG/lua/za/"

# Add to init.lua if not already present
INIT_FILE="$NVIM_CONFIG/init.lua"
if [ -f "$INIT_FILE" ]; then
    if ! grep -q 'require("za").setup()' "$INIT_FILE"; then
        echo -e "${YELLOW}Adding za setup to init.lua...${NC}"
        echo "" >> "$INIT_FILE"
        echo "-- Za syntax highlighting" >> "$INIT_FILE"
        echo "require('za').setup()" >> "$INIT_FILE"
    else
        echo -e "${GREEN}Za setup already found in init.lua${NC}"
    fi
else
    echo -e "${YELLOW}Creating new init.lua with za setup...${NC}"
    echo "-- Za syntax highlighting" > "$INIT_FILE"
    echo "require('za').setup()" >> "$INIT_FILE"
fi

echo -e "${GREEN}Installation complete!${NC}"
echo -e "${GREEN}Restart Neovim to activate za syntax highlighting.${NC}"
echo
