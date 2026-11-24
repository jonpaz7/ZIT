#!/usr/bin/env bash
set -e

echo "=== Installing ZIT CLI & Tracker ==="

INSTALL_DIR="${HOME}/.local/bin"
mkdir -p "$INSTALL_DIR"

go build -o "$INSTALL_DIR/zit" ./cmd/zit
go build -o "$INSTALL_DIR/zit-tracker" ./cmd/zit-tracker

echo "Installed zit and zit-tracker to $INSTALL_DIR"

if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo "Notice: Make sure $INSTALL_DIR is in your PATH."
    echo "Add this to your ~/.bashrc or ~/.zshrc:"
    echo '  export PATH="$HOME/.local/bin:$PATH"'
fi

echo "Done! You can now run 'zit' and 'zit-tracker'."
