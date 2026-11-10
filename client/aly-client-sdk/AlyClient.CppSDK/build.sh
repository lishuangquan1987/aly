#!/usr/bin/env bash
# 编译 C++ SDK + 一致性用例（MinGW-w64，C++11）。
set -euo pipefail
cd "$(dirname "$0")"
OUT="${1:-out}"
mkdir -p "$OUT"
g++ -std=c++11 -O2 -Wall -Wextra -D_WIN32_WINNT=0x0501 \
    -Iinclude src/json.cpp src/api.cpp src/client.cpp tests/conformance.cpp \
    -o "$OUT/aly-cpp-conformance.exe" \
    -static -static-libgcc -static-libstdc++
echo "Built $OUT/aly-cpp-conformance.exe"