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

- 14 unit tests in the `selfupdate` package (hash comparison, replacement orchestration, rename dance, rollback, journal);
- 4 tests for the `self_update` command in `cmd` (JSON protocol, lock semantics);
- tc end-to-end cases 130–135 (unified execution path, UI loop-guard, full finisher flow).
