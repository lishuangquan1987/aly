#!/usr/bin/env bash
set -euo pipefail

# Deploy the Linux updater + UI into an UpdateFolder/MainFolder pair.
# Usage: deploy-client-linux.sh <UpdateFolder> <MainFolder> [amd64|arm64]

if [ "$#" -lt 2 ]; then
  echo "usage: $0 <UpdateFolder> <MainFolder> [amd64|arm64]" >&2
  exit 2
fi

UPDATE="$1"
MAIN="$2"
ARCH="${3:-amd64}"
CLIENT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST="$CLIENT_ROOT/dist/linux-$ARCH"

test -f "$DIST/aly-client" || { echo "missing $DIST/aly-client; run build-linux.sh" >&2; exit 1; }
test -f "$DIST/aly-client-ui" || { echo "missing $DIST/aly-client-ui; run build-linux.sh" >&2; exit 1; }

mkdir -p "$UPDATE" "$MAIN"
install -m 0755 "$DIST/aly-client" "$UPDATE/aly-client"
install -m 0755 "$DIST/aly-client" "$MAIN/aly-client"
install -m 0755 "$DIST/aly-client-ui" "$UPDATE/aly-client-ui"
install -m 0755 "$DIST/aly-client-ui" "$MAIN/aly-client-ui"

echo "Deployed linux/$ARCH:"
sha256sum "$UPDATE/aly-client" "$UPDATE/aly-client-ui"
sha256sum "$MAIN/aly-client" "$MAIN/aly-client-ui"
echo "Start: $UPDATE/aly-client-ui -mode auto -lang zh-CN"