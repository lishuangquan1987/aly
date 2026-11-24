---
title: Troubleshooting
description: Look up by symptom — rejected publish, missing updater, update not triggered, stalled download, failed swap, held lock, server not starting. Each entry says which log to read and how to fix it.
---

# Troubleshooting

Answer two questions first: **which side is failing** (publishing machine, terminal, server) and **is there a log**. All three sides are covered below.

## Where the logs are

| Location | File | What it records |
|---|---|---|
| `UpdateFolder/` | `update.log` | check, apply, rollback and self-update execution |
| `UpdateFolder/` | `download.log` | downloads, materialising unchanged files, delete sync |
| `UpdateFolder/` | `check_diff.log` | local MD5 scan warnings |
| Publish GUI | `logs/log.log` | GUI log; the “Logs” button in the status bar opens the folder |

The three updater logs rotate as a single generation at 5 MB (`*.log.1`). The GUI picks the first writable log directory among: `logs/` next to `AlyPublish.exe` → `%LOCALAPPDATA%\AlyPublish\logs\` → `AlyPublish\logs\` in the temp directory.

## Publishing side

| Symptom | What to check | Fix |
|---|---|---|
| Banner says `aly-publish` was not found | `publish-cli/<rid>/` in the package, and `cli_rid` in `payload.json` | Reinstall AlyPublish, then click “Detect again” |
| No updater in `UpdateFolder/` after initialising | `client/<rid>/` missing from the package; product directory and main executable on different drives | Reinstall; when they are on different drives `main_exe_relative_path` cannot be resolved and the step fails |
| Publish rejected: version not greater than latest | The current latest version | Use a higher version; the first release must be at least `V1.0.1` (see [Version number rules](/en/docs/publish#version-number-rules)) |
| Publish failed: file was modified | MD5 recorded at staging time no longer matches disk | Refresh, stage again, publish |
| Server project list is empty | Server URL, and whether the server is running | Create a server project in the GUI, or fix the URL |
| Linux cannot find the GTK library | `libgtk-3.so.0` | `apt install libgtk-3-0` (or the equivalent for your distro) |
| CLI or updater taken from the package will not execute on Linux | zip drops the executable bit | `chmod 0755` |
| Directory already initialised, initialising again fails | — | Use “Add local project” instead |

## Terminal update side

| Symptom | What to check | Fix |
|---|---|---|
| `check_update` reports no update while the server has one | Local version in `version.json`; `project_name` and `server_url` in `shared.json` | Align project name and server URL; a client only accepts a **strictly higher** version |
| “No update” while the server is unreachable | Deliberate behaviour in the `downloaded` state | Query again once online; downloaded content is kept |
| Download stalls | `failed_files` in `download.log`; `.part` files | Re-run `download_update`; only failed files are retried, Range resumption applies |
| `apply_update` fails repeatedly | `update.log`; `apply_fail_count` in `version.json` | Clear the process holding the folder and retry; after the fuse trips, `check_update` stops pushing apply |
| Swap fails, folder is busy | Processes missing from `must_close_process_name` | Add them to `client.json`; the updater probes and kills the holder |
| Lock is held | The holder in `UpdateFolder/.aly.lock` | Make sure no update process runs; stale locks expire after 30 minutes TTL and 5 minute heartbeats |
| Odd behaviour after editing `version.json` by hand | State versus on-disk content mismatch | Never edit it; let `download_update` rewrite the state |

## Server

| Symptom | What to check | Fix |
|---|---|---|
| Exits at startup, log says `failed opening connection to sqlite` | The `-db` directory is not writable | Check owner and permissions |
| Exits at startup, log says `failed creating schema resources` | Database corrupted or in use | Stop the duplicate instance; restore from backup |
| Upload returns “file too large” | Single file above 1 GB, or proxy `client_max_body_size` | Raise the proxy limit, for example `client_max_body_size 1050m` |
| Proxy returns 413 | Nginx default of 1 MB | Same as above |
| Download returns 404 | `path` is not an absolute path under `data/`, or the binary moved | Request the `fileAbsolutePath` returned by `get_all_files` verbatim |
| Download breaks off mid-way | Each download sets a write deadline from the file size (`5 minutes + 1 second per MiB`, capped at 30 minutes); the global `WriteTimeout` is 0 | Very large files on slow links can still time out; the client resumes with Range retries |
| `database is locked` | Several processes open the same database | Run exactly one `aly-server` instance |
| Change-log timestamps differ from local time | `time` is sent by the publisher and stored verbatim | Standardise on `yyyy-MM-dd HH:mm:ss` and one time zone on the publishing side |

The server exposes no dedicated version endpoint; compare `GET /api/server/info`'s `version` (the Go runtime version) with the binary name or release record.

## Related

- [FAQ](/en/docs/faq): platforms, downtime, crash recovery, authentication
- [Host integration](/en/docs/client/): lifecycle, JSON contract, exit codes
- [Core concepts](/en/docs/concepts): state machine and offline semantics
- [Deployment & operations](/en/docs/server/deploy): systemd, reverse proxy, backups
