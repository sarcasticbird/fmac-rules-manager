#!/bin/bash
set -euo pipefail

BINARY_NAME="fmac-rules-manager-host"
MANIFEST_NAME="fmac_rules_manager.json"

# Detect OS
case "$(uname -s)" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux" ;;
  *)      echo "Unsupported OS"; exit 1 ;;
esac

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Copy binary
echo "Installing $BINARY_NAME to /usr/local/bin/"
sudo cp "$SCRIPT_DIR/../dist/$BINARY_NAME" "/usr/local/bin/$BINARY_NAME"
sudo chmod 755 "/usr/local/bin/$BINARY_NAME"

# Install manifest for each browser
install_manifest() {
  local dir="$1"
  local browser="$2"
  mkdir -p "$dir"
  cp "$SCRIPT_DIR/${MANIFEST_NAME%.json}.$OS.json" "$dir/$MANIFEST_NAME"
  echo "Installed manifest for $browser: $dir/$MANIFEST_NAME"
}

if [ "$OS" = "darwin" ]; then
  install_manifest "$HOME/Library/Application Support/Mozilla/NativeMessagingHosts" "Firefox"
  if [ -d "$HOME/Library/Application Support/zen" ]; then
    install_manifest "$HOME/Library/Application Support/zen/NativeMessagingHosts" "Zen"
  fi
else
  install_manifest "$HOME/.mozilla/native-messaging-hosts" "Firefox"
  if [ -d "$HOME/.zen" ]; then
    install_manifest "$HOME/.zen/native-messaging-hosts" "Zen"
  fi
fi

echo "Done. Restart your browser for the extension to detect the host."
