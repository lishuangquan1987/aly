#!/usr/bin/env bash
# 编译 Java SDK + 测试（目标 Java 8）。
set -euo pipefail
cd "$(dirname "$0")/.."
OUT="${1:-out}"
ARG_FILE="$(mktemp)"
trap 'rm -f "$ARG_FILE"' EXIT
find src -name '*.java' > "$ARG_FILE"
javac -encoding UTF-8 --release 8 -Xlint:-options -d "$OUT" @"$ARG_FILE"
echo "Java classes written to $OUT"