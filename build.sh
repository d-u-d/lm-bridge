#!/bin/bash
# Usage: ./build.sh [version]
# Example: ./build.sh v0.4.0
# Without argument — uses git tag or "dev"

set -e

cd "$(dirname "$0")"

export PATH="$PATH:$(go env GOPATH)/bin"

if [ -n "$1" ]; then
  VERSION="$1"
elif git rev-parse --git-dir > /dev/null 2>&1; then
  VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
else
  VERSION="dev"
fi

echo "Building lm-bridge $VERSION..."

wails build -ldflags "-X main.Version=$VERSION"

echo "✓ Built $VERSION"
echo "  $(ls -lh build/bin/lm-bridge.app/Contents/MacOS/lm-bridge | awk '{print $5}')"
