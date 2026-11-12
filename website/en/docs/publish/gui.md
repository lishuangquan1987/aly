---
title: Publish GUI (AlyPublish)
description: AlyPublish.exe's runtime requirements, project bookmarks, initialization and local projects, change review, publishing and version history, shortcuts, language switching and logs.
---

# Publish GUI (AlyPublish)

`AlyPublish.exe` is the Avalonia desktop front end (.NET 8) for `aly-publish`, aimed at release engineers who are not comfortable with the command line. It does not reimplement the publishing logic: every operation goes through `aly-publish.exe --json` in the same directory, so the `.updator/` config, the staging area and the version rules are exactly the same as the CLI's.

<ScreenshotFrame src="/images/publish-gui-overview.webp" alt="AlyPublish main window in English: project list on the left, unstaged and staged files in the middle, publish panel and version history on the right" caption="AlyPublish main window (English)" wide />

For the Chinese UI, see [publish-gui-overview-zh.webp](/images/publish-gui-overview-zh.webp).

## Requirements

| Requirement | Description |
|---|---|
| Executable | `AlyPublish(.exe)` |
| Required dependency | `publish-cli/<cli_rid>/aly-publish(.exe)` inside the package; `cli_rid` comes from the package's `payload.json` |
| Runtime | Release installer/portable zip is self-contained on .NET 8; a source `dotnet build` is framework-dependent |
| Minimum window | 1080 × 640; default 1320 × 840 |

Download the package (click a file name):

<ReleaseDownloads group="package" compact :show-latest-hint="false" :show-version="false" />

The in-package payload layout and platform selection are in [Download & Install](/en/docs/download). CLI lookup order:

1. `publish-cli/<cli_rid>/aly-publish(.exe)` named by `payload.json`;
2. The other platform directories in the package (fallback in `payload_rids` order; a warning is logged when a non-native build is used);
3. `aly-publish(.exe)` next to `AlyPublish` (the old manual layout still works);
4. The project directory, then the development tree's `publish/publish-cli/` (local development only).

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

- `DisplayName` is used only in the left-hand list; the search box matches both display names and paths.
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

- Find and copy `aly-client.exe` / `aly-client-ui.exe` from the GUI directory, the project directory or the development tree;
- Write `client.json` containing the main executable's relative path and the names of the processes to close;
- If the updater cannot be found, only a warning is shown and initialization is not blocked; a cross-drive setup fails because the main executable's relative path cannot be computed.

After initialization the project is added to the left-hand list and selected. It does **not** upload any files and does not create a server project automatically.

::: warning First publish version number
The server already occupies V1.0.0 when the project is created. After initialization, the first publish must use V1.0.1 or higher, or the anti-rollback check rejects it.
:::

## Adding a local project

"Add local project" takes over a directory that is **already initialized**:

- The directory must already contain `.updator/shared.json`;
- It reads and displays the server URL, the project name and the number of ignore rules;
- The display name defaults to the server project name, otherwise to the folder name;
- It only saves a bookmark: no files are copied, `.updator/` is not modified, and the CLI is not called.

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

## Reviewing changes

The upper half of the right-hand side is the changes area:

- The "Unstaged" table lists the additions and modifications returned by `status`; **deleted items are not shown**, and the GUI cannot stage deletions manually. Deletion is performed by the client, driven by the manifest frozen by the server after a release.
- The "Staged" table lists staging entries; the "Server" column of a staged entry is empty (`—`), because only the local snapshot is recorded when a file is staged.
- Columns: Select, File, Status, Local, Server. Status badges distinguish new / modified.
- Each row offers "Show in folder"; the top buttons support refresh, settings, opening the project directory and launching the main executable.
- The unstaged area provides "Select all / Unselect all", "Stage selected" and "Stage all"; the staged area provides "Select all / Unselect all", "Unstage" and "Clear" (clearing asks for confirmation).
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
| Release notes | Required; the GUI submits them as **one** log entry — for multiple entries use repeated `--message` with the CLI |
| Advanced | The post-update script (relative to the application directory) and the force-update checkbox |
| Publish button | The label follows the staged count ("Publish 3 files"); with nothing staged it prompts "Stage at least one file on the left first" |

Publishing process:

- The GUI invokes `push` with `--json`, parses START / DONE / FAIL line by line, and displays file names, sizes, statuses and overall progress;
- It can be cancelled; cancelling terminates the CLI process. Files already uploaded are not rolled back, and the staging area is not cleared;
- On success it clears the version, release notes, script and force-update checkbox, refreshes the status and the version history, and shows "Published: V1.0.2";
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

Translations use the JSON scheme of Irihi.Lingua:

| File | Purpose |
|---|---|
| `Resources/Strings.json` | Default (English) resources |
| `Resources/Strings.zh-Hans.json` | Simplified Chinese resources |

`LanguageManager` generates the key class and the observable strings at compile time through `[LinguaManager("./Resources/Strings.json")]`; XAML uses:

```xml
Text="{Translate {x:Static loc:LanguageManager+Keys.S009}}"
```

When adding new text, add the same key to both JSON files and then rebuild to generate the `Keys` member; switching language at runtime is driven by `UpdateCulture` and needs no restart.

## Logging and troubleshooting

The GUI writes file logs with Serilog. The files live in a `logs/` directory, roll daily, and are named like `log20261110.log` (multiple processes on the same day may add a suffix such as `_001`). The log directory is chosen in the following order, taking the first writable one:

1. `logs/` in the directory containing `AlyPublish.exe`;
2. `%LOCALAPPDATA%\AlyPublish\logs\`;
3. `AlyPublish\logs\` under the temp directory.

The "Logs" button in the status bar opens that directory directly.

| Symptom | Likely cause and fix |
|---|---|
| The banner says `aly-publish.exe` was not found | Put the CLI in the same directory as the GUI and click "Detect again" |
| The directory is already initialized, so initialization cannot continue | Use "Add local project" instead |
| The server project list is empty | "Create a server project" first, or check the server URL |
| Publish rejected: the version is not greater than the latest version | Use a higher version number; the first publish must be at least V1.0.1 |
| Publish fails and reports that files were modified | Refresh and stage them again; the CLI re-checks the MD5 recorded by add |
| Upload fails | Check the FAIL line in the progress panel and the log; the staging area is not cleared, so retry after fixing the problem |
| The UI language is wrong | Check `settings.json`; override it with `--lang zh` / `--lang en` |

## Building and distribution

The Release workflow now builds and uploads two GUI assets (see `.github/workflows/release.yml`):

- `AlyPublish-Setup.exe`: a Windows x64 installer, self-contained on .NET 8, bundling `aly-publish.exe`;
- `AlyPublish-win-x64.zip`: a portable bundle that runs after extraction.

The installer is produced with Inno Setup, installs into `Program Files\AlyPublish`, and creates Start Menu and optional desktop shortcuts. Neither artifact requires a separate .NET runtime on the target machine.

Build from source:

```bash
# Day-to-day debug build (framework-dependent; needs .NET 8 locally)
dotnet build publish/publish-gui/src/AlyPublish/AlyPublish.csproj -c Release

# Self-contained single file (same shape as the CI portable bundle)
dotnet publish publish/publish-gui/src/AlyPublish/AlyPublish.csproj \
  -c Release -r win-x64 --self-contained true \
  -p:PublishSingleFile=true -p:IncludeNativeLibrariesForSelfExtract=true \
  -o publish/publish-gui/artifacts/app
```

When distributing manually, keep `AlyPublish.exe` and `aly-publish.exe` in the same directory. If the initialization wizard should also generate `UpdateFolder` automatically, place `aly-client.exe` / `aly-client-ui.exe` in the GUI directory (or the project directory).

## Next steps

- CLI command details: [Config Commands](/en/docs/publish/commands-config), [Project Commands](/en/docs/publish/commands-project), [Workflow Commands](/en/docs/publish/commands-workflow)
- Publisher tool overview: [aly-publish](/en/docs/publish)
- Get the CLI and the server: [Download & Install](/en/docs/download)