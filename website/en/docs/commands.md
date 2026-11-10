# aly-client Commands

Every updater command outputs the same envelope:

```json
{ "isSuccess": true, "errorMsg": "", "data": { ... } }
```

Config precedence: `client.json` → `.updator/shared.json` → CLI flags. For the exhaustive parameter reference see [COMMANDS.md](https://github.com/lishuangquan1987/aly/blob/master/COMMANDS.md) in the repository.

## Command Overview

| Command | Purpose | Typical caller |
|---|---|---|
| `check_update` | Check whether a new version exists | Host app / GUI / SDK |
| `check_diff` | Compare local vs. server files (MD5+SHA256) | Debugging / advanced integrations |
| `download_update` | Download changed files (byte-level progress + materialization) | Host app / SDK |
| `apply_update` | Apply a downloaded update (atomic rename + restart) | Host app / SDK |
| `list_rollback_versions` | List rollback-able version snapshots | GUI / SDK |
| `rollback --version X` | Roll back to a version snapshot | GUI / SDK / ops |
| `clean [--keep N] [--all]` | Prune old snapshots and temp files | Scheduled tasks |
| `check_self_update` | Check whether the updater itself needs updating | SDK / GUI |
| `self_update` | Replace itself from the newer copy in the main folder | SDK / GUI delegation |

## Core Command Examples

### check_update

```bash
aly-client.exe check_update [--url <server>] [--project-name <name>]
```

```json
{ "isSuccess": true, "data": { "has_update": true, "need_download_update": true,
  "new_version": "1.0.1", "force_update": false } }
```

### download_update

```bash
aly-client.exe download_update
```

- Downloads only files that differ (manifest hash comparison)
- Per-file MD5 + SHA256 verification, resumable (`.part` temp files)
- A single file failing after 3 retries doesn't abort the rest; failures land in `failed_files`
- Unchanged files are materialized into the new version directory automatically

### apply_update

```bash
aly-client.exe apply_update
```

Flow: validate the downloaded package → close processes listed in `must_close_process_name` → three directory renames (backup → activate) → restart the main app. A crash at any moment recovers automatically on next startup — see [Crash Recovery](/en/docs/concepts#crash-recovery).

### rollback

```bash
aly-client.exe list_rollback_versions
aly-client.exe rollback --version V1.0.0
```

Rollback uses local version snapshots (`ApplicationFolder_V1.0.0/`) — no network needed. Refuses while an apply is unfinished; restores the prior state on failure.

### self_update

```bash
aly-client.exe self_update
```

Replaces the updater's own copy in `UpdateFolder` with the newer one shipped inside the main application folder: SHA256 comparison decides, a `.new` staging file + verification + atomic placement, and a rename dance (`.old` backup + rollback) for a running exe on Windows. Combined with `check_self_update` this closes the self-update loop — see [Self-Update Design](/en/docs/self-update).

## aly-client-ui Launch Modes

```bash
# Normal start (works with aly-client; bottom-right topmost window)
aly-client-ui.exe

# Check mode (no window; prints the check result JSON)
aly-client-ui.exe -mode check

# Linux self-update finisher (guardian mode invoked at UI startup)
aly-client-ui -finish-self-update --target <exe path> --pid <old PID> --created <starttime> --new <.new file> -- <restart command...>
```

The finisher verifies the target process identity (`starttime` from `/proc/<pid>/stat`, guarding against PID reuse), waits for it to exit (up to 10 s, then SIGKILL), places the `.new` file atomically and restarts the UI — no GTK required, runnable in headless environments.
