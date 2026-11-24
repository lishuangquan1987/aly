---
title: Publish your first release
description: Install one AlyPublish package, start the server, initialise the project in the GUI and publish V1.0.1 — four steps, every command copyable.
---

# Publish your first release

Goal: **have your first version on the server within ten minutes**. Install → start the server → initialise the project → publish.

::: tip Prerequisites
A machine that can run the server (your own laptop is fine) and the product directory you ship, for example `dist/`.
:::

## 1. Install AlyPublish

::: code-group

```powershell [Windows]
Invoke-WebRequest -Uri "https://github.com/lishuangquan1987/aly/releases/latest/download/AlyPublish-Setup.exe" -OutFile "AlyPublish-Setup.exe"
Start-Process ".\AlyPublish-Setup.exe"
```

```bash [Linux]
curl -fL -o AlyPublish-linux-x64.zip "https://github.com/lishuangquan1987/aly/releases/latest/download/AlyPublish-linux-x64.zip"
mkdir -p AlyPublish-linux-x64
unzip -q AlyPublish-linux-x64.zip -d AlyPublish-linux-x64   # the zip has no top-level folder
cd AlyPublish-linux-x64
chmod +x install.sh                                 # zip loses the executable bit
./install.sh
```

:::

One package gives you the publish GUI, the publish CLI and the terminal updater. On Windows you can also unzip `AlyPublish-win-x64.zip` and use it in place; asset names for arm64 and macOS are in [Download & install](/en/docs/download).

## 2. Start the server

```bash
./aly-server -p 2000
```

On Windows download `aly-server-windows-amd64.exe`, rename it to `aly-server.exe` and run it. The first run creates `aly.db` and `data/` next to the executable.

## 3. Initialise the project in the GUI

1. Start `AlyPublish` and click **Initialize project**;
2. Fill the server URL (for example `http://127.0.0.1:2000`) → **Fetch list** → select the project; if the server has none yet, click **Create server project**;
3. Fill the local product directory (for example `dist`), the display name and the **main executable** (this is what prepares terminal updates);
4. Confirm.

The GUI writes `.updator/` and creates a **sibling** `UpdateFolder/` next to your product directory, containing the updater and `client.json`.

## 4. Stage and publish

1. Back in the main window click **Refresh**;
2. **Stage all**;
3. Set the version to `V1.0.1` and write a change note;
4. **Publish**.

::: warning First version number
Creating a server project already occupies `V1.0.0`, so the first release must be `V1.0.1` or higher; the downgrade guard rejects anything else.
:::

Prefer the terminal? The same steps with the packaged CLI (`publish-cli/<rid>/aly-publish`):

```bash
aly-publish config init --server http://127.0.0.1:2000 --project myapp
aly-publish project create --server http://127.0.0.1:2000 --name myapp --title "My App"
aly-publish status
aly-publish add --all
aly-publish push --version V1.0.1 --message "First release"
```

## ✅ You're done

The server holds `V1.0.1` and the sibling `UpdateFolder/` is in place — the terminal machines can update themselves now.

## Next

- [Integrate the updater](/en/docs/start/integrate): check for updates when your app starts
- [Publish GUI](/en/docs/publish/gui): fields, shortcuts and log locations
- [Packaging & publishing](/en/docs/publish): day-to-day release flow and version rules
