---
title: Core Concepts
description: Precise semantics of the version state machine, atomic replacement, crash recovery, differential download, the update lock and configuration rules.
---

# Core Concepts

Understand these five mechanisms and you can explain how Aly behaves under power loss, network loss, repeated triggers and rollback.

## Version State Machine (version.json)

`UpdateFolder/version.json` records which stage of the update flow the client is in:

```text
(empty/applied) ──download_update────────→ downloaded ──apply_update──→ applying ──success──→ applied
   │                                          │                            │
   └── check_update finds a new version ──────┘                            └── crash → recovered on next startup
```

| Field | Meaning |
|---|---|
| `version` | Current / pending version |
| `version_previous` | Version of the actual content currently in the main directory (used for rollback and recovery) |
| `version_status` | Empty, `applied`, `downloaded`, `applying` |
| `apply_stage` | While `applying`, written as `activating` to mark that the backup rename has completed |
| `downloaded_file_count` | Number of regular files in the downloaded output; apply uses it to detect an incomplete directory |
| `rollback_previous` | Version of the actual main-directory content when the rollback started |
| `rollback_target` | Target version of the current rollback (used to resume an interrupted one) |
| `apply_fail_count` | Number of consecutive apply failures for the same pending version (circuit breaker) |
| `after_apply_update_script` | Script to run after a successful update (relative to the application directory) |

### State Guards

- `apply_update` refuses to replace when the state is empty or unknown; it rejects `version_previous == version`.
- Before applying, it validates the pending directory: if the directory is missing, empty, or holds fewer files than `downloaded_file_count`, it refuses and self-heals to `applied`, and the next round downloads again.
- `rollback` refuses while the apply is unfinished; on failure it restores the state from before the attempt instead of always writing `applied`.
- Once `apply_fail_count` reaches the threshold, `check_update` no longer reports "keep applying", so the process is not repeatedly killed and relaunched.

### Offline Semantics

When `check_update` is in the `downloaded` state and the server is unreachable, it returns `has_update=false` — no prompt when it cannot verify, the downloaded content is kept, and apply continues once the network is back. `applying` (crash recovery) and rollback resumption are exceptions: they must be reported even offline, otherwise the machine cannot heal itself.

## Atomic Replacement

The core of `apply_update` is **pure directory renames** — no copying, no deleting:

1. `download_update` **materializes** unchanged files from the current `ApplicationFolder` into the new version directory (the application is still running).
2. Rename `ApplicationFolder` → `ApplicationFolder_{previous_version}`.
3. Write `apply_stage=activating`, then rename `ApplicationFolder_{new_version}` → `ApplicationFolder`.
4. On failure, roll the state back to `downloaded` and clear the transient fields.

The downtime window is only three renames; even GB-scale directories take seconds.

## Crash Recovery

If `version_status=applying` at startup, the updater re-runs the atomic renames. `apply_stage=activating` is used to decide whether the rename actually happened; when an older version lacks this field, the updater falls back to a heuristic and logs a warning.

## Differential Download and Verification

- The publisher uploads only files that differ from the previous version; the client downloads only files whose local MD5/SHA256 differs.
- A single file that fails 3 times does not abort the whole run; the list of failures goes into `failed_files`, and a retry covers only the failed files.
- Large files use `.part` files for resumable transfers; the server's `download_file` supports Range.
- Version comparison is semver-aware: `1.0.0-beta < 1.0.0`, build metadata is ignored.

## Update Lock

`.aly.lock` guarantees that only one update flow runs at a time. The lock is valid while its holding process is alive; only genuinely stale leftover locks are cleaned. Concurrent triggers from the host, GUI and SDK are safe.

## Configuration Rules: ignore and un_copy

The two sets of rules take effect at different stages and must not be mixed up:

| Setting | Where it applies | Semantics |
|---|---|---|
| `ignore_folders` / `ignore_files` | **Server scan / publish manifest** | Decides which files are not delivered. Folders are **matched by plain prefix and are case-sensitive** (`logs` works, `logs/**` is not interpreted as a glob); files support exact, glob, filename and `*.ext` matches |
| `un_copy_folders` / `un_copy_files` | **Client materialization** | Everything is migrated by default; the listed paths are **not copied** into the new version directory; `*`/`**` are supported, exact names are case-insensitive |

::: warning Common misunderstanding
`ignore_*` is not applied again on the client during apply, and `un_copy_*` does not affect publisher uploads. A wrong rule raises no error; it only makes the update package larger or keeps runtime files from being migrated.
:::

## Cleanup (clean)

`clean` prunes old version snapshots, `.part`/`.tmp`/`.old*` files and expired logs. `--keep N` keeps the most recent N snapshots (`N < 1` falls back to the default of 3); `--all` keeps only the protection set for the current version. The most recent snapshot that rollback depends on is never deleted.