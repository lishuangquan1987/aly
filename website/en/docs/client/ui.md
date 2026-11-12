---
title: GUI Front-End aly-client-ui
description: The Windows and Linux forms of aly-client-ui, its launch modes, the -lang and -project-name parameters, single-instance forwarding, and the -finish-self-update finisher flow.
---

# GUI Front-End aly-client-ui

`aly-client-ui` is an optional graphical front-end for `aly-client`. It implements no update logic of its own; instead it launches the updater subprocess from the same directory (or from the main program directory), parses its JSON output, and shows check results, download progress and prompt buttons in a topmost card in the bottom-right corner of the screen.

<ScreenshotFrame src="/images/client-ui-update-en.webp" alt="aly-client-ui card in English when a new version is found" caption="Card when a new version is found: the version number, what changed in this update, and the “Update now” and “Later” buttons" />

The card does not steal focus and does not appear in the taskbar, so it suits full-screen operator software that needs a prompt on demand. When the update finishes, `aly-client` starts the main program and the UI process exits.

## Platforms and File Naming

| Platform | UI executable | Updater | Implementation |
|---|---|---|---|
| Windows | `aly-client-ui.exe` | `aly-client.exe` | Go 1.10.8 / 386, `lxn/walk`, Windows XP compatible, no console window |
| Linux | `aly-client-ui` | `aly-client` | Go 1.27 + MyGo 0.8.3 native UI, runtime dependency on GTK3, no WebKitGTK dependency |

Both read `UpdateFolder/client.json` and take the project name from `MainFolder/.updator/shared.json` as the title.

- On Windows, the updater is looked up in the UI's own directory first; if it is not found, the main program directory is derived from `client.json` and `aly-client.exe` is searched there.
- On Linux, only `UpdateFolder/aly-client` is used; the updater must sit in the same directory as the UI.

## Launch Modes

`-mode` defaults to `auto`. Unknown values fall back to `auto`; case and leading/trailing whitespace are normalized.

| Mode | Window behavior | Description |
|---|---|---|
| `auto` | Shown only when there is an update | Called when the main program starts. With no update, or when the check fails (offline / no server configured), it exits silently and never disturbs the user |
| `silent` | Same as `auto` | Semantically equivalent; kept to express "explicitly do not disturb" |
| `check` | Always shown | Called when the user clicks "Check for updates". It shows "Checking for updates…" first; with no update it says so explicitly (Windows shows a confirmation box, Linux closes itself after 5 seconds); on error it shows the reason plus retry/close |
| `update` | Reserved | The value is accepted. On Windows it currently shares the `auto` flow (shown only when there is an update); on Linux it follows the visible `check` flow. Do not rely on it to implement "silent forced update" |
| `rollback` | Always shown | Lists rollback-able versions for selection, then runs `rollback --version <v>` once one is chosen |

Platform differences:

- A Windows `auto` / `silent` instance checks in the background; if the user starts another instance with `-mode check` or `-mode update`, the latter forwards a "manual check request" and the first instance is promoted to interactive and shows its window.
- On Linux, a second instance forwards its full argument list to the first instance; the first instance shows and focuses its window, switches mode according to the arguments, and restarts the check or the rollback when necessary.

### Update Interaction

Non-forced updates use card buttons: no modal dialog, no focus stealing:

```text
check_update → new version found → [Update now] [Later]
             → download_update (byte progress + [Pause/Resume])
             → [Restart to finish] [Later]
             → apply_update → the main program is restarted
```

- **Forced update**: shows a 10-second countdown and restarts automatically; the user can click "Restart to finish" to end the wait early.
- **Pause/Resume**: pausing terminates the download subprocess and keeps the `.part` checkpoint; resuming starts the download again in a new session, and files already completed are not transferred twice.
- **Retry on failure**: the error card shows a plain-language reason (for example, a file is in use) and offers "Retry" and "Close"; retry goes back to the stage that failed and does not rerun stages that already succeeded.
- **Download progress**: byte-level progress (downloaded / total / speed) is preferred; when the updater is an older version without byte fields, it falls back to file counts.
- **Success**: shows "Update complete" for about 1.2 seconds and then exits; the main program is started by `aly-client`.
- `auto` / `silent` exit silently both when the check fails and when there is no update; an error or a no-update message appears only after forwarding has promoted the instance to interactive.

## Launch Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `-mode` | string | No | `auto` | `auto` / `check` / `update` / `rollback` / `silent` |
| `-lang` | string | No | `zh-CN` | `zh-CN` / `en-US`; unknown values fall back to Chinese |
| `-project-name` | string | No | Empty | Overrides the window/card title only; not passed to the updater |
| `-url` | string | No | Empty | **Reserved; currently has no effect**; the updater reads `shared.json` itself |

Go's `flag` package accepts both single-dash and double-dash forms, so `-mode` and `--mode` are equivalent.

### -lang

- Defaults to `zh-CN`; `en-US` selects English.
- Accepts case and underscore variants such as `en`, `en_us` and `EN-US`.
- It does not switch automatically with the system locale, and the UI has no in-app language switcher; the language is injected once at startup.
- Unknown values always fall back to Simplified Chinese.

<ScreenshotFrame src="/images/client-ui-update-zh.webp" alt="aly-client-ui update card in Chinese" caption="The Chinese card when launched with -lang zh-CN" />

### -project-name

Only decides the window and card title; it does not change the server project name and is not passed to `aly-client`. When omitted:

1. read `project_name` from `MainFolder/.updator/shared.json`;
2. if that cannot be read, fall back to `aly-client-ui`.

### -url

A retained parameter with no effect at present. The updater reads `server_url` from `shared.json` itself; to temporarily override the address, call `aly-client` with `--url` directly rather than relying on the UI.

### Examples

Windows:

```bat
:: Called when the main program starts (recommended)
..\UpdateFolder\aly-client-ui.exe -mode auto -lang zh-CN

:: The user clicks "Check for updates"
..\UpdateFolder\aly-client-ui.exe -mode check -lang zh-CN

:: Rollback UI
..\UpdateFolder\aly-client-ui.exe -mode rollback -lang en-US
```

Linux:

```sh
./aly-client-ui -mode auto -lang zh-CN
./aly-client-ui -mode check -lang en-US
./aly-client-ui -mode rollback -lang zh-CN
```

## Window Characteristics

| Platform | Size and position | Window attributes |
|---|---|---|
| Windows | 380×160 (at 96 DPI), anchored to the bottom-right of the work area (taskbar excluded), scaled with the system DPI; when there is update content the card grows taller automatically and repositions itself | `HWND_TOPMOST`, `WS_EX_TOOLWINDOW` (not in the taskbar, not in Alt-Tab), `SWP_NOACTIVATE` (no focus stealing) |
| Linux | 420×250 (minimum 380×210), anchored to the bottom-right of the primary display's work area | Borderless, non-resizable, `AlwaysOnTop`, `SkipTaskbar` |

## Single Instance

Only one UI instance is allowed per user session; a second instance exits immediately with exit code `0`.

- **Windows**: uses the session-named mutex `AlyClientUI_SingleInstance`. If the second instance runs with `-mode check` or `-mode update`, it notifies the first instance through the named event `AlyClientUI_CheckRequested`; the first instance shows its window and reruns the check as an interactive flow. Second instances in any other mode simply exit without forwarding.
- **Linux**: uses MyGo's `RequestSingleInstanceLock` and `OnSecondInstance`. The second instance's arguments are forwarded to the first instance; the first instance shows/focuses its window, switches mode according to the arguments, and restarts the check while idle, with no update, or in the error stage (in `rollback` mode it opens the rollback list instead).

Single-instance behavior not only keeps two bottom-right windows from covering each other, it also keeps two processes from replacing `aly-client` or the UI itself at the same time.

## Self-Update of the Updater and the UI

At startup the UI handles two self-updates in order; a failure of either does not block this launch:

1. **Updater self-update**: `UpdateUpdater` delegates `<UpdateFolder>/aly-client self_update` to copy the newer updater distributed with the version in `MainFolder` into `UpdateFolder`. It falls back to a direct copy only when the updater is missing or an older version does not recognize `self_update` (a process-level failure); when the lock is held (a JSON business failure) there is no fallback, so `.aly.lock` is never bypassed.
2. **UI self-update**: compares `MainFolder/aly-client-ui(.exe)` with `UpdateFolder/aly-client-ui(.exe)`. If they differ, the file is copied to `.new` and `.new` is spawned in its hidden finisher mode `-finish-self-update`; the old UI exits immediately, leaving the replacement and the restart to the finisher process. The `self_update` command only manages the updater and cannot manage the UI itself.

Startup self-updates keep an anti-loop journal:

| Platform | Journal file |
|---|---|
| Windows | `UpdateFolder/aly-client-ui.exe.selfupdate.json` |
| Linux | `UpdateFolder/aly-client-ui.selfupdate.json` |

If it fails to converge 3 times within the 30-minute observation window, the UI self-update is skipped for this launch (the UI still starts and checks for updates normally); the counter recovers automatically once the window has passed. Deleting the file resets the count immediately.

## -finish-self-update

`-finish-self-update` is the UI's hidden finisher mode, spawned by the old UI during a self-update; ordinary hosts never need to call it directly. It runs before the single-instance mutex and window creation; on Linux it also runs before MyGo/GTK initialization, so the finisher completes even without a graphical environment.

### Parameters

| Parameter | Type | Required | Description |
|---|---|---|---|
| `--target` | string | Yes | Absolute path of the old UI executable |
| `--pid` | int | Yes | Process ID of the old UI |
| `--created` | string | No | Identity check value for the old process: the decimal `FILETIME` creation time on Windows, the `starttime` from `/proc/<pid>/stat` on Linux; `0` or empty means unknown |
| `--host-pid` | int | No | PID of the original host process. When launching the new UI, the finisher appends it to the new UI's arguments so its host-following keeps watching the real host — otherwise the new UI's parent (the finisher) exits immediately and the host-following misjudges "host exited" after about 1 second and kills itself, so the auto-mode update card never appears |
| `--new` | string | Yes | Absolute path of the already-verified new UI file (usually `.new`) |
| `--` | — | No | Everything after it is passed through verbatim to the restarted new UI |

Constraints:

- `--target` and `--new` must be absolute paths and must differ; `--new` must exist.
- If `--target`, `--new` or `--pid` is missing, or an unknown argument appears, the finisher process exits with code `2`.
- Argument parsing does not use the `flag` package, because everything after `--` has to be passed through verbatim; only the five options above are accepted before `--`.

### Flow

1. **Wait for the old process to exit**:
   - Windows: open a handle to the old process and call `WaitForSingleObject` for up to 10 seconds; verify the PID against the process creation time, and if it was reused treat the old process as exited and never kill the wrong one; on timeout, `TerminateProcess` and wait 2 more seconds.
   - Linux: poll `/proc/<pid>`, detect PID reuse with `starttime`, and treat a zombie (`state=Z`) as exited; up to 10 seconds, then `SIGKILL` and wait 2 more seconds.
2. **Abort when the old process state cannot be confirmed** (exit code `1`): better not to replace at all than to modify files while the old process still holds the lock and launch a new UI that would collide with the single-instance mutex.
3. **Back up first, then replace**: if the target exists, rename it to `.old`; then rename `.new` to the target. Renames retry 20 times at 250 ms intervals, covering brief antivirus locks; if placement fails, rename `.old` back to the target.
4. **Launch the new UI**: start the target with the original arguments after `--` (the finisher appends `--host-pid <original host>` and strips any stale `--host-pid` left in the restart arguments), without going through `cmd.exe`, so there are no PATH, quoting or `%` expansion problems.
5. **Write the log**: append to `<target>.update.log` — `aly-client-ui.exe.update.log` on Windows, `aly-client-ui.update.log` on Linux.
6. Exit code `0` on success; exit code `1` if waiting, replacement or launching fails.

Example (constructed automatically by the old UI; no need to run it by hand):

```bat
aly-client-ui.exe -finish-self-update ^
  --target C:\App\UpdateFolder\aly-client-ui.exe ^
  --pid 4321 ^
  --created 133456789012345678 ^
  --host-pid 1234 ^
  --new C:\App\UpdateFolder\aly-client-ui.exe.new ^
  -- -mode check -lang zh-CN
```

## Exit Codes

| Exit code | Scenario |
|---|---|
| `0` | Normal completion; a second instance exiting immediately; the finisher replaced and restarted successfully |
| `1` | The finisher could not confirm the old process state, the replacement failed, or launching the new UI failed |
| `2` | Window/path initialization failed; the finisher received invalid arguments |

## Related Pages

[Command reference](/en/docs/commands) · [Configuration reference](/en/docs/client/config) · [Updater overview](/en/docs/client/) · [Self-update design](/en/docs/self-update)