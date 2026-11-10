# Core Concepts

This page explains the four pillars of Aly's reliability: the version state machine, atomic replacement, crash recovery and differential downloads.

## Version State Machine (version.json)

`UpdateFolder/version.json` records which stage of the update flow the client is in; every command works around it:

```text
applied ──(download_update)──→ downloaded ──(apply_update starts)──→ applying
                  │                        │
                  │                        ├──(success)──→ applied
                  │                        └──(crash)──→ detected on next startup → auto-recover
                  │
                  └──(check_update)──→ has_update=true (server reachable)
```

### State Guards

Writes happen only on consistent states — refuse rather than corrupt:

- `apply_update` **refuses to replace** when `version_status` is empty or unknown (empty means "no pending update", exactly like `applied`); it also refuses `version_previous == version` (backup and version directories would collide).
- `apply_update` validates the downloaded package first: a missing, empty, or under-counted `ApplicationFolder_{version}` (fewer files than the `downloaded_file_count` recorded at download time) refuses the replace and self-heals to `applied` — the next `check_update` re-downloads. It never "succeeds" with a copy of the current folder or a mixed old/new package.
- `rollback` refuses to run while an apply is unfinished, and restores the pre-rollback status on failure instead of always writing `applied`.

### Offline Semantics

`check_update` only reports a pending apply while the server is reachable: `downloaded` + unreachable server returns `has_update=false`, so the host never shows a "ready to update" prompt that disagrees with the online state. Nothing is discarded — `apply_update` runs as soon as the server is reachable again. `applying` is the deliberate exception: crash recovery and rollback resumption are **always reported**, even offline — otherwise a machine missing its application folder could never recover by itself.

## Atomic Replace

The heart of `apply_update` is **pure directory renames** — no copying, no deleting; GB-scale folders complete in seconds:

1. During `download_update`, after all files are downloaded and verified, unchanged files are **materialized** from the current `ApplicationFolder` into the new version directory — while your app is still running; read-only copies don't lock the rename that follows.
2. Rename `ApplicationFolder` → `ApplicationFolder_{previous_version}` (backup).
3. Write the `apply_stage=activating` marker, then rename `ApplicationFolder_{new_version}` → `ApplicationFolder`.
4. If any rename fails, revert `version_status` to `downloaded` so it can be retried, and clear transient fields.

Because materialization happens at download time, the app is only down for three renames — from minutes (full-copy schemes) down to seconds.

## Crash Recovery

If startup finds `version_status` in `applying`, the updater **re-executes the atomic rename step** — an update is never left in a broken half-state. The `apply_stage=activating` marker written right before the final rename decides whether the rename already happened (legacy states fall back to the old heuristic and log a warning).

## Differential Download & Verification

- `download_update` compares MD5 + SHA256 against the server manifest and **downloads only changed files**, with byte-level progress.
- Single-file failures retry 3 times without aborting the whole download — failed files are collected in `failed_files`; a retry downloads only the failures, already-verified files are skipped.
- Version comparison is semver-aware: `1.0.0-beta` sorts before `1.0.0` (pre-release < release); plain numeric versions keep the legacy behavior.

## Server-Side Downgrade Guard

`publish_version` rejects a version that is not strictly greater than the project's latest — preventing accidental republishing of an old version number that would leave updated clients "stuck". To roll content back, publish a higher version number.

## Update Lock

`.aly.lock` guarantees a single update flow at any moment: live holders are identified by real process-liveness probing (not timestamp guessing), so only genuinely stale locks get cleaned. Concurrent triggers from GUI, host and SDK are all safe.

## Cleanup (clean)

The `clean` command prunes old version snapshots, temp files (`.part`/`.tmp`/`.old*`) and stale logs; `--keep N` controls how many snapshots to keep, `--all` clears everything. Snapshots rollback depends on are never pruned away.
