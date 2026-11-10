# FAQ

## Why still support Windows XP?

Many production environments (industrial HMIs, POS terminals, hospital equipment) still run XP — and those machines are exactly where safe updates matter most. Mainstream updaters (Squirrel, Velopack, Electron updater) all require Windows 7+. Aly keeps that path alive by pinning Go 1.10.8 + 386, with CI validating XP compatibility (PE header + manifest) automatically.

## Does the host application have to be C#?

No. `aly-client` is a standalone executable; every command prints the same stdout JSON protocol (`{"isSuccess", "errorMsg", "data"}`). Any language that can spawn a subprocess and parse JSON can host it. The C# SDK merely saves you the orchestration.

## How long is the app down during an update?

The `apply_update` downtime window is **three directory renames** — seconds. Unchanged files are materialized into the new version directory at download time (while the app still runs), so the apply stage copies nothing. GB-scale directories take the same seconds.

## What happens if the power is cut mid-update?

Nothing bad. All replacement is pure directory renames guarded by the `version.json` state machine: on next startup an `applying` state re-executes the rename step; the downloaded package is validated before applying (file count, content version) — an incomplete one self-heals and re-downloads. See [Crash Recovery](/en/docs/concepts#crash-recovery).

## How does the updater update itself?

Through the unified `self_update` executor: the new updater rides the version package into the main application folder; at startup the GUI/SDK delegate to the updater itself, which takes the lock, compares SHA256, stages through `.new`, rolls back on failure, and circuit-breaks after 3 consecutive failures within 30 minutes. See [Self-Update Design](/en/docs/self-update).

## Does rollback need network?

No. `rollback` uses local version snapshots (`ApplicationFolder_V1.0.0/`) — pure local renames. Snapshot retention is controlled by `clean --keep N`.

## Is there authentication on the server?

The current version is designed for a **trusted intranet**; the API ships without built-in authentication or signing. For public exposure, add authentication at a gateway. See [Server API · Security Notes](/en/docs/api#security-notes).

## Are updates incremental?

Yes. The publisher uploads only files that differ from the previous version; the client downloads only files that differ locally (MD5+SHA256 comparison). Large files use chunked upload with resume; a single-file failure never aborts the rest — retries fetch only the failures.

## What does the Linux GUI need installed?

Only the GTK3 runtime that desktop distros preinstall. The UI library MyGo is pure Go (no cgo); artifacts are single ELF files and arm64 cross-compiles directly. Headless machines can still run the `-finish-self-update` finisher (no GTK required).

## How do I migrate or change the server address?

Change `server_url` and publish again: `.updator/shared.json` ships with the version, and clients swap in the new config automatically when they download the update.
