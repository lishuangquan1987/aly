---
title: Integrate the updater
description: Three steps to make an existing app self-update — confirm UpdateFolder, launch the updater from your host, run one full update locally.
---

# Integrate the updater

Goal: **have your app check for and apply updates at startup**. This assumes you already published at least one release; if not, read [Publish your first release](/en/docs/start/publish) first.

## 1. Make sure UpdateFolder/ exists

Initialising the project in the GUI already creates it. The layout is:

```text
PackageFolder/
├── ApplicationFolder/                 # the app itself (current version)
│   └── .updator/shared.json           # server URL and project name
└── UpdateFolder/
    ├── aly-client(.exe)               # the updater
    ├── aly-client-ui(.exe)            # optional update window
    └── client.json                    # main executable and processes to close
```

Without one, copy both files from `client/<rid>/` of the package into `UpdateFolder/` and write `client.json`:

```json
{
  "main_exe_relative_path": "../ApplicationFolder/YourApp.exe",
  "must_close_process_name": ["YourApp"]
}
```

`main_exe_relative_path` is resolved **relative to `UpdateFolder/`**. Field reference: [Configuration files](/en/docs/client/config).

## 2. Launch the updater from your host

::: code-group

```csharp [C# SDK]
using AlyClient.CSharpSDK;

var ui = new AlyUpdateUIClient();   // defaults to ..\UpdateFolder\aly-client-ui.exe
ui.LaunchAuto();                    // silent unless an update is available
```

```bash [CLI / any language]
cd UpdateFolder
./aly-client check_update      # line-delimited JSON; trust isSuccess
./aly-client download_update   # changed files only
./aly-client apply_update      # close, swap, restart
```

:::

In any other language, launch `aly-client` as a child process and read stdout line by line: skip progress lines carrying `data.status`, and judge the final result by `isSuccess`. A business failure can still exit 0, so never go by the exit code. Full contract: [Command reference](/en/docs/commands#common-conventions).

## 3. Verify locally

Run the three commands above once: the app should be closed, its folder swapped, and the new version started again.

## ✅ You're done

Your app checks for updates at startup; when one exists, `apply_update` closes it, swaps the directory and restarts it.

## Next

- [Host integration](/en/docs/client/): lifecycle, progress lines and failure handling
- [Client command reference](/en/docs/commands): every flag and return field
- [Troubleshooting](/en/docs/troubleshooting): update not triggered, swap failing
