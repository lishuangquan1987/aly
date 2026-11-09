#!/usr/bin/env bash
set -euo pipefail

# Build Linux updater (aly-client) + MyGo native UI for amd64/arm64.
# Requires Go 1.27.3 on PATH or ALY_GO127.

CLIENT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ALY_CLIENT_ROOT="$(cd "$CLIENT_ROOT/../aly-client" && pwd)"
UI_ROOT="$CLIENT_ROOT/aly-client-linuxui"
DIST="$CLIENT_ROOT/dist"
GO="${ALY_GO127:-go}"

"$GO" version
mkdir -p "$DIST"

tmp_gp="$(mktemp -d)"
trap 'rm -rf "$tmp_gp"' EXIT
mkdir -p "$tmp_gp/src/aly/client"
cp -a "$ALY_CLIENT_ROOT" "$tmp_gp/src/aly/client/aly-client"

for arch in amd64 arm64; do
  out="$DIST/linux-$arch"
  mkdir -p "$out"
  (cd "$tmp_gp/src/aly/client" && \
    GOPATH="$tmp_gp" GO111MODULE=off GOOS=linux GOARCH="$arch" CGO_ENABLED=0 \
    "$GO" build -trimpath -o "$out/aly-client" aly/client/aly-client)
  chmod 0755 "$out/aly-client"
done

cd "$UI_ROOT"
"$GO" tool mygo vet
"$GO" test ./...
"$GO" tool mygo build -platform linux/amd64,linux/arm64

for arch in amd64 arm64; do
  src="$UI_ROOT/build/linux-$arch/aly-client-ui"
  [ -f "$src" ] || { echo "missing $src" >&2; exit 1; }
  out="$DIST/linux-$arch"
  mkdir -p "$out"
  cp -f "$src" "$out/aly-client-ui"
  chmod 0755 "$out/aly-client-ui"
done

echo "Built:"
find "$DIST" -maxdepth 3 -type f -print