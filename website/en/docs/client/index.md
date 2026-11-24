---
title: Host integration
description: How a host launches the updater, how to read its JSON, why child processes exit with the host, plus the update lock, security boundaries and rollback.
---

# Host integration

`aly-client` does not stay resident and installs no service; the host launches it when needed. Each run executes one subcommand and writes the result to stdout as JSON. The actual version switch is carried out by local directory renames — no network is involved, and the main program does not participate in replacing files.

For the shortest working path see [Integrate the updater](/en/docs/start/integrate); this page covers what comes after. The updater is only responsible for turning the local files into the version the server published; when to prompt the user, and whether an update is forced, belongs to the host or the GUI.

## Components and their source

| Component | Role |
|---|---|
| `aly-client(.exe)` | The updater itself: check, download, apply, roll back, self-update |
| `aly-client-ui(.exe)` | Optional update window wrapping the same commands and showing progress |
| `AlyClient.CSharpSDK` | Wrapper for .NET hosts: background state machine, events, static API, GUI launcher |
| `aly-server` / `aly-publish` | Store version metadata and files, publish new versions |

Both updater binaries come from `client/<rid>/` inside the AlyPublish package: the GUI copies them into `UpdateFolder/` when a project is initialized, or you place them by hand. For the directory and `<rid>` mapping see [Package layout & distribution](/en/docs/guide/package).

## Host lifecycle: exit together with the host

`aly-client` and `aly-client-ui` are child processes launched by the host. **They exit when the host exits** and never stay in the process table after the user closes the main program.

The only exception is the **apply stage**: the updater has to close the host process, atomically replace the main directory and restart the main program, so the host is going to (and must) disappear anyway — the updater has to finish that step instead of exiting halfway. Two conditions qualify, either one is enough:

- Command level: the current command is `apply_update`;
- State level: `version_status == applying` in `UpdateFolder/version.json` (the crash-recovery window — no command is running then, but the main directory may already be gone, so it must resume in the same way).

Exit is a graceful shutdown: write the log first, then exit. The update lock needs no explicit release — a lock file is judged stale by “the owning process has exited”, and the next run cleans up any leftover lock automatically. The normative text is `spec/host-contract.md` §15.

## Recommended call order

```text
startup
 ├─ self_update when needed (bare console deployments)
 ├─ check_update
 │   ├─ has_update=false            → done
 │   ├─ need_download_update=true   → download_update → apply_update
 │   └─ need_download_update=false  → apply_update (crash recovery / downloaded and pending)
 └─ exit
```

Every flag and return field is in the [command reference](/en/docs/commands).

## Five rules for reading the JSON

- **Trust `isSuccess`, never the exit code.** Business failures (failed download verification, no pending update, a lock held by another process) also end with exit code `0`; only an unknown or missing subcommand returns `1`, and a parameter parsing failure returns `2`.
- **Progress lines are not the final result.** The `BEGIN` / `START` / `PROGRESS` / `DONE` / `FAIL` lines of `download_update` all carry `data.status`; the final completion line has `data` set to `null`. Only `data:null` means the download finished.
- **Do not splice parameters into `cmd.exe`.** Pass arguments directly to avoid quoting, spacing and `%` expansion problems.
- **The updater reads its own configuration.** Hosts normally do not pass `--url` / `--project-name`; `client.json` and `MainFolder/.updator/shared.json` must be in place.
- **Close the processes.** Put the main program's process name into `must_close_process_name` in `client.json`, or pass it through the SDK's `MustCloseProcessNames`; otherwise, when a rename fails, the updater probes for and kills the holder and the host never gets a chance to shut down gracefully.

If you need a graphical interface, use `aly-client-ui` instead of drawing your own UI — see [Update window](/en/docs/client/ui).

## Processes and the lock

- The global lock `.aly.lock` guarantees a single update flow per endpoint at any moment. It is valid while the owning process lives; stale leftovers are cleaned automatically after a 30-minute TTL with 5-minute heartbeats. Concurrent triggers from a host, the GUI and the SDK are safe.
- Never edit `version.json` by hand. A mismatch between the state and the disk content makes the updater refuse to replace; the only repair is rewriting the state through `download_update`.
- The apply stage is the one exception to “exit with the host”, see the lifecycle section above.

## Security boundaries

- The server REST API **has no authentication**. Deploy it on a trusted intranet or behind a gateway; never expose it directly to the public internet.
- The updater has write access to `MainFolder` and `PackageFolder`; restrict ordinary users' write access on the endpoint so version directories or `version.json` cannot be tampered with locally.
- Downloaded files are verified against the MD5 + SHA256 in the server manifest, but the manifest itself comes from the server: the security model is “trust your own server”, not end-to-end signing.

## Rollback

- Every successful apply renames the old main directory to a `ApplicationFolder_{version_previous}` snapshot; `rollback --version` swaps directories directly and needs no network.
- The `apply_update` / `rollback` entry points opportunistically keep the 3 most recent snapshots; `clean` reclaims them explicitly. The current version, the previous version and the rollback target are never cleaned up.
- Rollback is not “version pinning”: if the server version is still higher after a successful rollback, the next `check_update` upgrades again. There is no rollback pin policy yet, so operations must unpublish or adjust the server version in parallel.
- A rollback failure after entering the replacement stage restores the pre-rollback state and starts the current version of the main program if the main directory still exists, so a machine is never left in the “state applied, disk still old” mismatch.

## Related

- [Integrate the updater](/en/docs/start/integrate): three steps to a working update
- [Client command reference](/en/docs/commands) · [Configuration files](/en/docs/client/config)
- [Update window](/en/docs/client/ui) · [C# SDK integration](/en/docs/sdk)
- [Core concepts](/en/docs/concepts) · [Troubleshooting](/en/docs/troubleshooting)
