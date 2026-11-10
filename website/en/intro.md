# Introduction

Aly is a **self-contained auto-update system for desktop applications**: the publisher pushes a new version with one command, and the updater on user machines completes the upgrade with a crash-safe atomic replacement. No third-party update framework is involved — server, publish tools, updater, GUI and SDK all live in this one repository.

## What Problems It Solves

Four common pain points of shipping desktop apps, and how Aly answers each:

| Pain point | Aly's answer |
|---|---|
| Old machines can't run update agents | The updater is pinned to the Go 1.10.8 + 386 toolchain and **runs on Windows XP**; CI validates the artifact PE headers automatically |
| A crash mid-update bricks the install | Pure directory renames + a version state machine; on startup an `applying` state is detected and recovered |
| Large updates keep the app down too long | Unchanged files are materialized at download time; apply performs only three renames — downtime is seconds |
| How does the updater update itself | The `self_update` command + GUI/SDK delegation + a failure-count journal break the loop, fully solved in-repo |

## Five Components

| Component | Role | Stack |
|---|---|---|
| **aly-server** | Stores project metadata & files, serves the REST API | Go 1.25 · Gin · Ent · SQLite |
| **aly-publish** | Publish CLI: init configs, scan diffs, stage, push | Go 1.25 · cobra |
| **aly-publish-gui** | Desktop publisher wrapping aly-publish, for non-technical publishers | Avalonia 11.3 · .NET 8 |
| **aly-client** | The updater executable, running on end-user machines | Go 1.10.8 (XP track) / modern Go (cross-platform track) |
| **aly-client-ui** | Optional GUI front-end: bottom-right, topmost, on demand | Windows: walk (XP) · Linux: MyGo + GTK3 |

Host applications drive the updater in two ways: spawn `aly-client` subprocesses and parse stdout JSON, or integrate the [C# SDK](/en/docs/sdk) which orchestrates check → download → apply → self-update automatically.

## Design Principles

- **Single file, zero dependencies**: the updater and GUI are single executables — copy and run, no runtime installation (the Linux GUI only needs GTK3, preinstalled on desktop distros).
- **Protocol over implementation**: all commands output `{"isSuccess": bool, "errorMsg": string, "data": ...}`; GUI, SDK and updater are decoupled, so any language can host.
- **Configuration as data**: publisher and client share `.updator/shared.json`, and that config ships with every version — server address changes roll out with the next update automatically.
- **Everything verifiable**: downloads are verified with MD5 + SHA256; version publishing is monotonic (the server rejects older version numbers); [135 end-to-end cases](/en/docs/testing) cover crash, power-loss and concurrency scenarios.

## When to Use It

- Intranet desktop apps (ticketing, POS, industrial HMI) — self-host the server, no public internet needed.
- .NET / C++ applications with many DLLs and assets — directory-level diffing, only changed files travel.
- Legacy systems that must stay on Windows XP / Windows 7 — something mainstream updaters (Squirrel, Velopack, Electron updater) cannot do.
