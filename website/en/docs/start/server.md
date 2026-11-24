---
title: Deploy the server
description: Download one binary, start it, verify the API answers — a usable aly-server in ten minutes. Production hardening lives in deployment & operations.
---

# Deploy the server

Goal: **a usable server within ten minutes**. Process supervision, reverse proxies and backups are covered in [Deployment & operations](/en/docs/server/deploy).

## 1. Download

::: code-group

```powershell [Windows]
Invoke-WebRequest -Uri "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-server-windows-amd64.exe" -OutFile "aly-server.exe"
```

```bash [Linux]
curl -fL -o aly-server "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-server-linux-amd64"
chmod +x aly-server
```

:::

Asset names for arm64 and macOS are in [Download & install](/en/docs/download). A single statically linked binary: no runtime, no external database service.

## 2. Start it

```bash
./aly-server -p 2000
```

The first run creates `aly.db` and `data/` next to the executable; uploaded files always live in `data/{project}/`. `-db` moves the database file.

## 3. Verify

```bash
curl http://127.0.0.1:2000/api/project/get_all_projects
```

An `isSuccess:true` response means the server is up (with no projects yet, `data` is an empty array).

## ✅ You're done

The server accepts publish and update requests at `http://<host>:2000`.

## Next

- [Publish your first release](/en/docs/start/publish): ship a version right away
- [Deployment & operations](/en/docs/server/deploy): systemd, reverse proxy, backups and upgrades
- [Server API](/en/docs/api): endpoint index and error semantics
