---
title: Testing
description: Aly's 148 end-to-end cases, unit-test coverage and how to run them locally.
---

# Testing

Aly tracks end-to-end cases in its repository test case document, `aly测试用例文档.md`. There are **148** of them, numbered consecutively from 1 to 148; each records its own "preconditions → steps → expected result → failure criteria" and can be reproduced independently. The most recent full regression result is whatever that document records; PASS counts are not frozen on this site. The document is currently Chinese-only; its numbered cases mirror test/tc cases 1..148, which are readable as Go code.

## E2E Framework (test/tc)

A Go-driven harness, not a patchwork of scripts:

- Each case is a standard `go test` function and shares the `tc` package's environment orchestration (start the server, build the client, lay out directories, assert on JSON);
- Dual-toolchain aware: the XP track builds the fake with Go 1.10.8, whereas the modern track builds the server; paths can be overridden with `ALY_GO126` / `ALY_GO1108`;
- Covers normal flows, crash injection, resumable transfers, rollback, the concurrency lock, the unified self-update path, and every publish CLI/GUI command.

| Group | Coverage |
|---|---|
| Normal flow | The full check → download → apply chain |
| New features | Stall timeout, failed-file retry, semver comparison |
| Crash and recovery | `applying` recovery, power-loss injection, state guards |
| Rollback and cleanup | All rollback paths, clean snapshot retention |
| Concurrency and locking | Two instances triggering an update concurrently |
| Self-update | Unified execution entry point, UI/SDK delegation, loop prevention, the Linux finisher process |
| Publishing tools | All commands, chunked-upload boundaries, atomic initialization writes, publish GUI |

## Unit Tests

| Package | Coverage |
|---|---|
| `selfupdate/` | Hash comparison, replacement orchestration, the rename dance for a running exe, rollback, Journal loop prevention |
| `cmd/` | JSON protocol of each command, lock semantics, script execution |
| `config/` / `util/` / `model/` | Configuration loading, platform utility functions |
| `linuxui` | Finisher process, replacement orchestration, argument parsing |
| `publish-gui` tests | Configuration, versioning, CLI output parsing, payload probing and more (54) |
| `server/controllers` | API boundaries: path traversal, chunk boundaries (0 / -1 / 65537), zero-byte merge, MD5 mismatch, concurrent merges, Range/If-Range, soft-delete filtering, idempotent delete, changelog cap (~40 cases) |
| `publish-cli` (api/diff/staging) | Chunked upload and complete retries, ignore-rule normalization, deleted staging and manifest criterion, force-update tri-state |
| SDK conformance | 17-18 cases per language (TS / Rust / C++ / Pascal 18, Python 17, Java 17, Go all green; C# covered by E2E 78-94) |

Test code runs on **modern Go (1.27) in GOPATH mode** and may use `t.Cleanup` / `t.TempDir`; production binaries must still be built with Go 1.10.8 / 386 for XP compatibility, so CI is designed as "build with Go 1.10, test with modern Go".

## Running

```bash
# All E2E (needs both Go toolchains)
cd test/tc && go test ./...

# A single case
cd test/tc && go test -run TestCase130 ./...

# Updater unit tests (Linux)
cd client/aly-client && go test ./cmd/ ./config/ ./util/ ./selfupdate/

# Publish GUI
cd publish/publish-gui && dotnet test tests/AlyPublish.Tests/AlyPublish.Tests.csproj
```

## CI

`release.yml` runs both tracks in parallel:

- Windows runner: XP-track build (386) + PE/manifest validation + server/publishing tool matrix build;
- Ubuntu runner: Linux client build (amd64/arm64) + Linux unit tests + publish GUI tests.