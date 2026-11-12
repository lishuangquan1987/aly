# Self-Update Design

"How does the updater update itself?" is the classic blind spot of auto-update systems: you must replace a running executable, survive power cuts, and never spin into a loop. Aly's self-update is built around **detection separated from execution**.

## Core Design: One Source of New Updater Versions

No component ever downloads "the updater itself" from the server. New versions travel the main update chain:

1. The publisher packs `aly-client` (the updater) as an ordinary file into the version package;
2. `download_update` fetches it like any other file (MD5+SHA256 verified);
3. `apply_update` atomically replaces the whole main application folder — which now contains the **new updater**;
4. Self-update merely syncs the "running old copy" in `UpdateFolder` from the main folder's new version.

Source integrity is guaranteed by the main chain; the self-update layer only compares and moves.

## Division of Labor Across Components

| Component | Role |
|---|---|
| `check_self_update` command | **Detection only**: compares its own copy with the main-folder copy by SHA256 and prints a `need_update` JSON — no replacement |
| `self_update` command | **The unified executor**: the updater itself takes `.aly.lock` and replaces the UpdateFolder copy from the main folder (GUI/SDK both delegate here) |
| GUI / SDK | **Orchestration**: detect at startup; on `need_update=true` delegate to `self_update`; local copying remains a fallback only when the updater is missing or outdated |

The replacement itself is implemented in the portable `selfupdate` package (shared by both build tracks):

- SHA256 comparison decides whether anything needs to happen (identical → skip);
- A `.new` staging file, verified after copying, then placed atomically;
- On Windows, a target locked by a running updater goes through a **rename dance**: rename the running exe to a `.old` backup → place the new one → roll back on failure; the target path never disappears;
- Leftover `.new`/`.old` files are cleaned at startup;
- **Failure-count journal**: ≥3 consecutive failures within a 30-minute window skip self-update — killing the "fail → restart → fail again" loop at the root.

## The Two-Piece Set: `aly-client` and `aly-client-ui` Converge Together

The updater folder really holds a **pair**: `aly-client(.exe)` (it performs the download/replace) and
`aly-client-ui(.exe)` (the GUI front-end). The release puts both into the main application folder
(the `deploy` scripts, the publish GUI's `UpdateFolderService` and `deploy-client-linux.sh` all use
that "same-named pair" layout), yet the copies in `UpdateFolder` converge through different paths:

| Copy | Who converges it | Semantics |
|---|---|---|
| `aly-client(.exe)` | `self_update` (the updater itself) | SHA256 convergence: identical → skip, different → rename dance, source missing → skip |
| `aly-client-ui(.exe)` | **The UI's own startup self-update** (`UpdateSelfUI` + `-finish-self-update`) | On a difference it copies `.new` and spawns the finisher: wait for the old UI to exit → rename the old exe to `.old` → place `.new` → restart the new UI |
| `aly-client-ui(.exe)` (**when missing**) | `self_update --include-ui` | Places it from the main folder **only when it is missing or 0 bytes** (bootstrap); an existing copy is never touched |

Why two paths for the UI copy: replacing a **running UI image** must be finished by the UI itself
(it has to exit, be replaced, and restarted), while "the UI copy is not there at all" has no process
able to do that — and that is exactly the field failure: the host's
`AlyUpdateUIClient.LaunchAuto()` only got `aly-client-ui.exe not found`. `--include-ui` exists for
that path:

- target missing/0 bytes → `ui_reason=replaced` (placed from the main folder, no `.old`);
- target exists → `ui_reason=existing_kepped` (**never replaces a possibly running image**);
- `<ui>.selfupdate.json` with `pending=true` (a finisher in flight) → `ui_reason=pending_inflight` (give way, do not queu);
- no such copy in the main folder → `ui_reason=source_missing` (not an error).

Both platforms use the **same names** (Windows `aly-client-ui.exe`, Linux `aly-client-ui`); the
source name is derived from the target name, so this path has no platform branch, and placement
reuses the `selfupdate` package's "staging file + SHA256 verify + atomic place (old target kept on
failure)" primitives — including the executable bit on Linux.

## Host Side: Make the Pair Present Before Launching the UI

The C# SDK's `AlyUpdateUIClient` performs one "pair deployment" step inside `Launch*`:

1. the UI copy already hashes identically to the main-folder copy → do nothing (no process spawned);
2. the updater copy is missing → place it from the main folder (otherwise the command cannot run);
3. the UI copy is missing/unreadable/outdated → delegate to `self_update --include-ui`;
4. the command is unusable, or the copy is still missing after it → place the main-folder copy directly;
5. at most one attempt per instance; failures return a diagnosable NG (never an exception).

**No loop can form**: `--include-ui` never restarts anything and only bootstraps a missing copy, so
convergence is a fixpoint (`existing_kepped`/identical afterwards); the UI's own replacement loop is
still guared by its journal (≥3 non-converging attempts within 30 minutes → skip).

## The Linux UI Finisher Process

Replacing a **running GUI** on Linux is handled by the `-finish-self-update` finisher mode:

```bash
aly-client-ui -finish-self-update \
  --target /path/aly-client-ui \
  --pid 12345 --created <starttime> \
  --new /path/aly-client-ui.new \
  -- <restart command and args...>
```

Flow: detach from the session (setsid) → verify the target process identity (`starttime` from `/proc/<pid>/stat`, guarding against PID reuse) → wait for it to exit (up to 10 s, then SIGKILL + 2 s grace) → place the `.new` file atomically → restart the new UI. Everything is logged to `aly-client-ui.update.log`, and GTK is not required — it runs in headless scripts.

## Testing Contract

The behavior contract of the self-update chain is pinned by three test layers (see [Testing](/en/docs/testing)):

- unit tests in the `selfupdate` package (hash comparison, replacement orchestration, rename dance,
  rollback, journal, and `SyncFile` pair deployment: missing-copy bootstrap / existing-copy kepp /
  finisher in flight / source missing / executable bit);
- `self_update` command tests in `cmd` (JSON protocol, lock semantics, the four `--include-ui` reasons);
- tc end-to-end cases 130–137 (unified execution path, UI loop-guard, full finisher flow,
  `--include-ui` pair deployment, and the SDK's auto-deployment before launching the UI, sd17).
