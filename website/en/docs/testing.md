# Testing

Aly manages all end-to-end cases in **one authoritative document** in the repository (`aly测试用例文档.md`, currently v11): 135 cases, continuous numbering, each self-contained with "preconditions → steps → expected results → failure criteria". Current status: **131 PASS / 0 FAIL / 1 SKIP / 3 not executed**.

## The E2E Framework (test/tc)

A Go-driven test harness — not a pile of PowerShell scripts:

- Every case is a standard `go test` function sharing the `tc` package's environment orchestration (start the server, build the client, lay out directories, assert JSON output);
- Dual-toolchain aware: the harness builds XP-profile client fakes with Go 1.10.8 and the server with a modern Go (toolchain paths overridable via `ALY_GO126` / `ALY_GO1108`);
- Case groups: normal flows (check/download/apply), late-stage features, crash recovery, resumable downloads, rollback, concurrency locks, unified self-update, publish-cli/gui full command coverage.

## Case Distribution (excerpt)

| Group | Cases | Covers |
|---|---|---|
| Normal flow | 1–8 | full check → download → apply chain |
| Late-stage features | 9–17 | stall timeout, failed-file retry, semver comparison |
| Crash & recovery | — | `applying` state recovery, power-loss injection, state guards |
| Rollback & cleanup | — | rollback paths, clean snapshot retention |
| Concurrency & locks | — | mutual exclusion of concurrent update triggers |
| Self-update | 130–135 | unified `self_update` execution, UI/SDK delegation & fallback, loop guard, finisher process |
| publish-cli / gui | — | all commands, chunked-upload boundaries, initialization atomic writes |

## Unit Tests

Beyond E2E, core packages carry standalone unit tests (running on both Linux and Windows):

| Package | Count | Covers |
|---|---|---|
| `selfupdate/` | 14 | hash comparison, replacement orchestration, running-exe rename dance, failure rollback, loop-guard journal |
| `cmd/` | 40+ | per-command JSON protocol, lock semantics, script execution |
| `config/` / `util/` / `model/` | 60+ | config loading, platform helpers |
| linuxui | 20 | finisher process, replacement orchestration, argument parsing |

Tests under the XP-track constraint keep Go 1.10.8 compatibility too (UI tests avoid `t.Cleanup` and newer APIs), so tests and artifacts share one profile.

## Tests in CI

`release.yml` runs both tracks in parallel:

- **Windows runner**: XP-track build (386) + PE header/manifest validation + matrix builds of server & publish tools;
- **Ubuntu runner**: Linux client builds (amd64/arm64) + `mygo vet` + full Linux unit tests.

## Running It Yourself

```bash
# All E2E (needs Go 1.10.8 and a modern Go; Linux defaults to PATH and ~/sdk/go1.10.8)
cd test/tc && go test ./...

# One case (example: unified self-update execution path)
cd test/tc && go test -run TestCase130 ./...

# Client unit tests (Linux)
cd client/aly-client && go test ./cmd/ ./config/ ./util/ ./selfupdate/
```
