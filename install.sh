#!/usr/bin/env bash
set -euo pipefail

REPO="kingjethro999/goo"
BINARY="goo"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# Get latest release version from GitHub API
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "Error: Could not determine latest version from GitHub API"
  exit 1
fi

echo "Installing ${BINARY} v${VERSION} for ${OS}/${ARCH}..."

# Download tarball
TARBALL="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${TARBALL}"

TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

curl -fsSL "$URL" -o "${TMP}/${TARBALL}"

# Verify checksum
curl -fsSL "https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt" \
  -o "${TMP}/checksums.txt"

cd "$TMP"
sha256sum --check --ignore-missing checksums.txt

# Extract and install
tar -xzf "${TARBALL}" -C "$TMP"
sudo install -m 755 "${BINARY}" "${INSTALL_DIR}/${BINARY}"

echo ""
echo "✓ ${BINARY} installed to ${INSTALL_DIR}/${BINARY}"
echo ""
echo "Get started:"
echo "  goo config set-key groq"
echo "  goo chat"
