---
title: Quick Start
description: Run Aly in three steps — install one AlyPublish package, initialize and publish from the GUI, then let the terminal machines update themselves.
---

# Quick Start

The goal is the **shortest working path**: install one package, click through the GUI to ship your first
version, then let the terminal machines update themselves. Equivalent CLI commands are given per step.

::: tip Prerequisite: the server must be running
Both “publish a release” and “integrate the updater” assume a running server. If you do not have one yet,
start it as in [Server deployment](/en/docs/server/deploy), or look at step 0 below.
::

<QuickStartWizard />

## 0. Start the server (once)

```bash
./aly-server -p 2000
```

The first run creates `aly.db` and `data/` next to the executable; uploaded files always live in
`data/{project}/`. On Linux use `./aly-server-linux-amd64`, on Windows `.\aly-server.exe`; `-db` moves the
database file.

## 1. Install AlyPublish: one package with GUI + CLI + updater

On Windows run `AlyPublish-Setup.exe`, or unzip `AlyPublish-win-x64.zip` and use it in place; on Linux unzip
`AlyPublish-linux-x64.zip` and run `./install.sh`. The full download table and the package layout are in
[Download & Install](/en/docs/download).

What you get:

```text
AlyPublish(.exe)                        # publish GUI
publish-cli/<rid>/aly-publish(.exe)     # publish CLI (the GUI drives it)
client/<rid>/aly-client(-ui)            # terminal updater (the GUI copies it into UpdateFolder/)
payload.json                            # the GUI picks its builds from it
```

## 2. Initialize and publish from the GUI (recommended)

1. Start `AlyPublish` and click **Initialize project**;
2. Step one picks the server project: fill the server URL (for example `http://127.0.0.1:2000`) → fetch the
   list → select it; when the server has no project yet, click **Create server project**;
3. Step two takes the local folder (the product directory you ship, for example `dist`), the display name and
   the **main executable** (optional, but it is what prepares the terminal update);
4. On confirm the GUI calls `aly-publish config init ...`, writes `.updator/`, and creates the **sibling**
   `UpdateFolder/`: it copies `aly-client` and `aly-client-ui` from `client/<rid>/` of the package and writes
   `client.json`;
5. Back in the main window: **Refresh** → **Stage all** → fill the version and the change note → **Publish**.

::: warning First version number
Creating the server project already occupies `V1.0.0`, so the first push must be `V1.0.1` or higher;
otherwise the downgrade guard rejects it.
::

The equivalent CLI path (inside the product directory):

```bash
aly-publish config init --server http://127.0.0.1:2000 --project myapp
aly-publish project create --server http://127.0.0.1:2000 --name myapp --title "My App"
aly-publish status
aly-publish add --all
aly-publish push --version V1.0.1 --message "First release"
```

The `aly-publish` you call is the one inside the package at `publish-cli/<rid>/` (on Linux `chmod 0755` it
after taking it out).

## 3. The layout on a terminal machine

```text
PackageFolder/
├── ApplicationFolder/              # the app that is currently live
│   ├── YourApp.exe
│   └── .updator/shared.json        # server_url, project_name, ignore rules
├── ApplicationFolder_V1.0.0/       # previous version snapshot (for rollback)
└── UpdateFolder/
    ├── aly-client(.exe)            # the updater
    ├── aly-client-ui(.exe)         # optional GUI front-end
    ├── client.json                 # main exe path and processes to close
    └── version.json                # version state machine (kept automatically)
```

`main_exe_relative_path` in `client.json` is resolved **relative to `UpdateFolder/`**:

```json
{
  "main_exe_relative_path": "../ApplicationFolder/YourApp.exe",
  "must_close_process_name": ["YourApp"]
}
```

## 4. The client checks and applies

```bash
aly-client check_update      # is there a new version: has_update / new_version / force_update
aly-client download_update   # downloads only changed files, MD5 + SHA256 verified
aly-client apply_update      # closes the app, replaces atomically, restarts it
```

A business failure usually still exits 0; rely on `isSuccess` in the JSON. Every field is in the
[command reference](/en/docs/commands).

## 5. Hosts: no orchestration needed (optional)

A C# host hands the whole update chain to the packaged window with one line:

```csharp
using AlyClient.CSharpSDK;

var ui = new AlyUpdateUIClient();   // defaults to ..\UpdateFolder\aly-client-ui.exe
ui.LaunchAuto();                    // silent check at startup; the window only shows when needed
```

The updater comes from the package (or the GUI), so the SDK no longer downloads it. Other hosts:
[language SDKs](/en/docs/sdk/languages).

## Next

- Every client command and its fields: [aly-client command reference](/en/docs/commands)
- GUI fields, shortcuts and troubleshooting: [Publish GUI](/en/docs/publish/gui)
- Crash safety and the state machine: [Core concepts](/en/docs/concepts)
- When something breaks: [FAQ](/en/docs/faq)
