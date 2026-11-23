---
title: Publish GUI (AlyPublish)
description: AlyPublish.exe's runtime requirements, project bookmarks, initialization and local projects, change review, publishing and version history, shortcuts, language switching and logs.
---

# Publish GUI (AlyPublish)

::: info Implementation and historical build
This page describes the **primary** Go / MyGo build (`publish-gui-go`, pure-Go native UI). The historical
C# / Avalonia build (`publish-gui-csharp`) is kept in the repository but is no longer built or released by
default: select `gui=csharp` in the Release Build workflow (or set the repository variable `PUBLISH_GUI`
to `csharp`) to produce the **same four asset names**. Both share the same `payload.json` and installer
scripts with identical config files; see [Download & Install](/en/docs/download).
:::

`AlyPublish.exe` is the Go / MyGo native UI desktop front end for `aly-publish`, aimed at release engineers who are not comfortable with the command line. It does not reimplement the publishing logic: every operation goes through `aly-publish.exe --json` in the same directory, so the `.updator/` config, the staging area and the version rules are exactly the same as the CLI's.

<ScreenshotFrame src="/images/publish-gui-overview.webp" alt="AlyPublish main window in English: project list on the left, unstaged and staged files in the middle, publish panel and version history on the right" caption="AlyPublish main window (English)" wide />

For the Chinese UI, see [publish-gui-overview-zh.webp](/images/publish-gui-overview-zh.webp).

## Requirements

| Requirement | Description |
|---|---|
| Executable | `AlyPublish(.exe)` |
| Required dependency | `publish-cli/<cli_rid>/aly-publish(.exe)` inside the package; `cli_rid` comes from the package's `payload.json` |
| Runtime | Single Go binary: no extra runtime dependency on Windows; GTK3 (`libgtk-3.so.0`) required on Linux, with a warning from `install.sh` when missing |
| Minimum window | 1080 × 640; default 1320 × 840 |

Download the package (click a file name):

<ReleaseDownloads group="package" compact :show-latest-hint="false" :show-version="false" />

The in-package payload layout and platform selection are in [Download & Install](/en/docs/download). CLI lookup order:

1. `publish-cli/<cli_rid>/aly-publish(.exe)` named by `payload.json`;
2. The other platform directories in the package (fallback in `payload_rids` order; a warning is logged when a non-native build is used);
3. `aly-publish(.exe)` next to `AlyPublish` (the old manual layout still works);
4. The `ALY_PUBLISH` environment variable, the project directory, then the development tree's `publish/publish-cli/` (local development only).

When `payload.json` is missing or invalid, the GUI falls back to platform detection (Windows → `windows-amd64`, Linux → `linux-amd64` / `linux-arm64`) and logs a line explaining why.

When the CLI cannot be found, an "aly-publish.exe not found" banner appears at the top of the window and offers two buttons, "Detect again" and "Open app folder"; once it is found, a green "CLI ready" badge appears in the upper-right corner. Every CLI-dependent operation (refresh, staging, publishing, project settings) fails while it is missing, and project initialization cannot complete either.

When calling the CLI, the GUI's working directory is the **project directory**, and the arguments always include `--json`. Per-operation timeouts: 120 seconds for a status refresh, 60 seconds for ordinary operations, 300 seconds for publishing.

## Project bookmarks

The left-hand list is a set of **bookmarks** that stores only the display name, the project path and the main executable path — not the server URL or the ignore rules, which live in `.updator/shared.json` inside the project directory.

Bookmark file on Windows:

```text
%LOCALAPPDATA%\AlyPublish\config.json
```

Example content:

```json
[
  {
    "DisplayName": "演示应用",
    "ProjectPath": "C:\\Aly\\Demo\\demo-endlist",
    "MainExePath": "DemoApp.exe"
  }
]
```

- `DisplayName` is used only in the left-hand list; the search box matches both display names and paths (Ctrl+F focuses the search box).
- `MainExePath` may be a path relative to the project directory; when the executable is outside the project directory, only an absolute path can be stored.
- "Remove" deletes the bookmark only and asks for confirmation; it does not delete project files or `.updator/`.
- Once a project is selected, the right-hand side reads `status` and the 20 most recent version log entries; the first load shows "Reading project status…".

## Initializing a project

"Initialize project" is a two-step wizard that binds an **existing server project** to a local directory that has not been initialized yet.

### Step 1: Select a server project

- The server URL can be chosen from addresses that have appeared in past bookmarks, or typed manually;
- Click "Fetch projects" to load the list, then filter it by project name or title;
- After selecting one you can continue to the next step, which shows the project's current version ("Not published" when nothing has been released);
- If the server has no projects yet, you can click "Create a server project" directly: enter the title and name (the name follows the title by default and stops syncing once it has been edited manually), force update and ignore rules. On success the dialog returns to the list with the new project selected.

### Step 2: Local directory and rules

| Field | Description |
|---|---|
| Local project directory | The build output directory; browse for it or drop the folder onto the window |
| Display name | Alias in the left-hand list; defaults to the server project name |
| Main executable | Optional; used by the "Launch" button and by `UpdateFolder` |
| Processes to close before updating | Defaults to the main executable's file name without the extension |
| Ignored folders/files | Skipped during the publish scan; type and press Enter, or pick them from the project directory |
| Folders/files not to copy | Skipped when applying an update, keeping the original files on the target machine |

Directory validation:

- It must exist;
- It must not already contain `.updator/shared.json` (in that case use "Add local project");
- It must not already be a bookmark in the left-hand list;
- The display name must not be empty.

On confirmation, the GUI calls:

```bash
aly-publish config init --server <server-url> --project <name> \
  --ignore-folders <comma-separated> --ignore-files <comma-separated> \
  --un-copy-folders <comma-separated> --un-copy-files <comma-separated>
```

If a main executable was provided, an `UpdateFolder/` is also created **next to** the project directory (as a sibling):

- The updaters are taken from the **package payload** `client/<client_rid>/` (the platform is decided by the package's `payload.json`) and written unconditionally;
- `client.json` is written containing the main executable's relative path and the names of the processes to close;
- If the updater cannot be found, only a warning is shown and initialization is not blocked; a cross-drive setup fails because the main executable's relative path cannot be computed;
- To ship updaters for another platform, take them from the package's other `client/<rid>/` directories (see [Download & Install](/en/docs/download)).

After initialization the project is added to the left-hand list and selected. It does **not** upload any files and does not create a server project automatically.

::: warning First publish version number
The server already occupies V1.0.0 when the project is created. After initialization, the first publish must use V1.0.1 or higher, or the anti-rollback check rejects it.
:::

## Adding a local project

"Add local project" takes over a directory that is **already initialized**:

- The directory must already contain `.updator/shared.json`;
- It reads and displays the server URL, the project name and the number of ignore rules;
- The display name defaults to the server project name, otherwise to the folder name;
- It only saves a bookmark: no files are copied, `.updator/` is not modified, and the CLI is not called;
- If the project already has a sibling `UpdateFolder/`, the two updaters inside are refreshed to the package payload's files with the same names (`client.json` is untouched); without an `UpdateFolder/` nothing happens.

The same directory cannot be added twice; a directory that is already a bookmark reports "This project is already in the list".

## Project settings

"Project settings" on the toolbar, or Ctrl+E, opens the settings dialog, which edits two kinds of configuration at once:

| Category | Fields | Written to |
|---|---|---|
| Local bookmark | Display name, main executable | `%LOCALAPPDATA%\AlyPublish\config.json` |
| Project config | Server URL, project name, ignore/no-copy rules | `.updator/shared.json` (through the CLI) |

Behavior:

- Before saving it re-reads `shared.json` and sends only the items that actually changed to the CLI (`config set` / `config set-array`), so a stale snapshot cannot overwrite changes made elsewhere.
- As soon as at least one item in `shared.json` changes, the local ignore rules are synced to the server project while the server-side title and force-update flag are preserved; a failed sync only warns and does not block the local save.
- Changing the project name does not rename the server project; it only makes the client connect under the new name, so a server project with that name must already exist.
- An empty server URL or project name prevents saving.
- When saving with a main executable and no sibling `UpdateFolder/` yet, the folder is created (the same code path as initialization: copy updaters from the package payload + write `client.json`); an existing `UpdateFolder/` is left alone to avoid overwriting user configuration.

## Reviewing changes

The upper half of the right-hand side is the changes area:

- The "Unstaged" table lists the additions and modifications returned by `status`. Files that exist only on the server are treated as **kept by the server**: they are not a difference, no "deleted" rows are shown, and publishing **never deletes server files** (staging entries left over from older builds that carried the deleted status are ignored with a warning).
- The "Staged" table lists staging entries; the "Server" column of a staged entry is empty (`—`), because only the local snapshot is recorded when a file is staged.
- Columns: Select, File, Status, Local, Server. Status badges distinguish new / modified.
- Each row offers "Show in folder"; the top buttons support refresh, settings, opening the project directory and launching the main executable.
- The unstaged area provides "Select all / Unselect all", "Stage selected" and "Stage all"; the staged area provides "Select all / Unselect all", "Unstage" and "Clear" (clearing asks for confirmation).
- Unstaged and staged are **fixed-ratio panes** (half each by default, draggable, remembered per project); the bottom of each table shows an "N files · total size" summary. On narrow windows the table drops the "Server" column automatically.
- Selecting a project for the first time and any manual refresh both re-read `status` and the log; the status bar at the bottom shows "N unstaged, M staged", the project path and the log shortcut.

## Publishing and version history

The publish panel on the right requires three conditions to be met at once:

1. The staging area is not empty;
2. The version number is not empty;
3. The release notes are not empty.

| Control | Description |
|---|---|
| Version | For example `V1.0.2`; when empty the button is disabled and shows "Enter a version" |
| Next version | Increments the last numeric segment of the current version by one (`V1.0.2` → `V1.0.3`); disabled when the current version is missing or cannot be parsed |
| Release notes | Required; **one entry per line** (blank lines are ignored), equivalent to the CLI's repeated `--message`. Long notes scroll inside the input and never squeeze the version history |
| Advanced | The post-update script (relative to the application directory) and the force-update checkbox (checked = on, unchecked = off; written explicitly to the server on publish) |
| Publish button | The label follows the staged count ("Publish 3 files"); with nothing staged it prompts "Stage at least one file on the left first" |

Publishing process:

- The GUI invokes `push` with `--json`, parses START / DONE / FAIL line by line, and displays file names, sizes, statuses and overall progress;
- It can be cancelled; cancelling terminates the CLI process. Files already uploaded are not rolled back, and the staging area is not cleared;
- On success it clears the version, release notes and script; the force-update checkbox is **refilled from the server's actual state** (it is no longer hard-reset to unchecked), then the status and version history refresh and "Published: V1.0.2" is shown;
- Force update is tri-state: sending `--set-force-update` when checked and `--clear-force-update` when unchecked; neither is sent only when the server value was never read (keep the current value and avoid clearing it by mistake);
- On failure it shows the error message; the staging area is preserved and you can retry directly. If the files were uploaded but version registration failed, retry with a higher version number.

The version history shows the 20 most recent entries, returned by the server with the newest first:

- The newest entry in the history is marked with a highlighted badge; the current version also appears in the "Changes" heading and in the version badge of the publish panel;
- The V1.0.0 "第一次创建" entry written when the server created the project also appears in the history;
- With no entries it shows "No releases yet".

## Keyboard shortcuts

| Shortcut | Action |
|---|---|
| F5 | Refresh the current project (same as the "Refresh" button) |
| Ctrl+Enter | Publish the current project (triggered only when the publish button is enabled) |
| Ctrl+E | Open the settings for the current project |
| Ctrl+F | Focus the search box on the left |

## UI language

The language button in the upper-right corner shows the **target language**: the Chinese UI shows `EN`, the English UI shows `中文`.

- Clicking it switches immediately and writes the choice to `%LOCALAPPDATA%\AlyPublish\settings.json`:

  ```json
  {"language":"zh-Hans"}
  ```

- Language priority at startup: the command-line `--lang` > the saved `settings.json` > the operating-system UI language.
- A `--lang` value that starts with `zh` means Chinese; anything else means English:

  ```bash
  AlyPublish.exe --lang zh
  AlyPublish.exe --lang en
  ```

- `--lang` only affects the current launch and is not written to the settings file.

Translations are built into the program (Go key tables such as `i18n.go`, one zh/en pair), switchable at runtime with no rebuild. The theme (light/dark) and accent color are stored in `appearance.json` in the same directory, independently of the language setting.

## Logging and troubleshooting

The GUI writes file logs with the Go standard library: it appends to `log.log` and mirrors the same lines to stderr. The log directory is chosen in the following order, taking the first writable one:

1. `logs/` in the directory containing `AlyPublish.exe`;
2. `logs/` in the config directory (`%LOCALAPPDATA%\AlyPublish\logs\` on Windows);
3. `AlyPublish\logs\` under the temp directory.

The "Logs" button in the status bar opens that directory directly.

| Symptom | Likely cause and fix |
|---|---|
| The banner says `aly-publish.exe` was not found | The package's `publish-cli/<rid>/` is missing or `payload.json` does not match: reinstall AlyPublish and click "Detect again"; in the old manual layout a CLI next to the GUI is still recognized |
| No updater in `UpdateFolder` after initialization | The package is missing `client/<rid>/` (reinstall); if the main executable and the project directory are on different drives, `main_exe_relative_path` cannot be computed and the operation fails |
| Linux startup reports a missing GTK library | Install GTK3 (e.g. `apt install libgtk-3-0`); `install.sh` warns when the library is missing |
| The CLI/updater taken from the package cannot be executed on Linux | The zip does not preserve the executable bit: `chmod 0755`; `install.sh` does this for you |
| The directory is already initialized, so initialization cannot continue | Use "Add local project" instead |
| The server project list is empty | "Create a server project" first, or check the server URL |
| Publish rejected: the version is not greater than the latest version | Use a higher version number; the first publish must be at least V1.0.1 |
| Publish fails and reports that files were modified | Refresh and stage them again; the CLI re-checks the MD5 recorded by add |
| Upload fails | Check the FAIL line in the progress panel and the log; the staging area is not cleared, so retry after fixing the problem |
| The UI language is wrong | Check `settings.json`; override it with `--lang zh` / `--lang en` |

## Building and distribution

The Release workflow builds and uploads **four GUI assets** (see `.github/workflows/release.yml`):

- `AlyPublish-Setup.exe`: a Windows x64 installer bundling the GUI, `publish-cli/`, `client/` and `payload.json`;
- `AlyPublish-win-x64.zip`: a portable bundle that runs after extraction;
- `AlyPublish-linux-x64.zip`, `AlyPublish-linux-arm64.zip`: Linux packages (with `install.sh` / `uninstall.sh` inside); after extraction run `chmod +x install.sh` first (the zip does not preserve the executable bit), then `./install.sh` to install into `/opt/aly/AlyPublish` (`--prefix` can override; `install.sh` also restores the 0755 bits on the payload executables).

The publish CLI and the terminal updaters are **not** uploaded as standalone assets: they ship only inside the GUI package (one package is all a user needs).

The payload is staged by the inline pwsh step in the release workflow (`Stage the package payload`): it lays out the downloaded per-platform artifacts into the layout above, copies the platform's `payload.json`, and asserts that all 10 items (9 binaries + the manifest) are present (any miss fails the build). For local development `publish/deploy-publish.bat` copies the same file names into the GUI output directory.

Build from source:

```bash
cd publish/publish-gui-go

# Tests and static checks
go test ./... -count=1
go tool mygo vet

# Windows single file (-H windowsgui removes the console window)
go build -trimpath -ldflags "-H windowsgui" -o build/windows-amd64/AlyPublish.exe .

# Linux cross-compile (same source, switch GOOS/GOARCH; CI adds install.sh and zips on top)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o build/linux-amd64/AlyPublish .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o build/linux-arm64/AlyPublish .
```

(Or run `build.ps1` / `build.sh` directly: one run of fmt/vet/test plus all three platform binaries.)

When distributing manually, keep `AlyPublish.exe` together with `payload.json`, `publish-cli/` and `client/` in the same directory (the initialization wizard picks the updaters from that layout).

The historical C# build from source (kept, no longer released by default):

```bash
cd publish/publish-gui-csharp && dotnet build
```

## Next steps

- CLI command details: [Config Commands](/en/docs/publish/commands-config), [Project Commands](/en/docs/publish/commands-project), [Workflow Commands](/en/docs/publish/commands-workflow)
- Publisher tool overview: [aly-publish](/en/docs/publish)
- Get the CLI and the server: [Download & Install](/en/docs/download)
