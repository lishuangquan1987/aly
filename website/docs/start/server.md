---
title: 部署服务端
description: 下载一个二进制、跑起来、验证接口通；十分钟让 aly-server 可用，生产化另见部署与运维。
---

# 部署服务端

目标：**十分钟内让服务端可用**。进程托管、反向代理与备份见[部署与运维](/docs/server/deploy)。

## 1. 下载

::: code-group

```powershell [Windows]
Invoke-WebRequest -Uri "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-server-windows-amd64.exe" -OutFile "aly-server.exe"
```

```bash [Linux]
curl -fL -o aly-server "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-server-linux-amd64"
chmod +x aly-server
```

:::

arm64 与 macOS 的资产名见[下载与安装](/docs/download)。单个静态二进制，不需要运行时，也不需要外部数据库服务。

## 2. 启动

```bash
./aly-server -p 2000
```

首次运行在程序旁创建 `aly.db` 与 `data/`；上传的文件固定放在 `data/{项目}/`。`-db` 可以改数据库文件路径。

## 3. 验证

```bash
curl http://127.0.0.1:2000/api/project/get_all_projects
```

返回 `isSuccess:true` 就说明服务端通了（还没有项目时 `data` 是空数组）。

## ✅ 完成

服务端在 `http://<机器地址>:2000` 接受发布与更新请求。

## 接下来

- [发布第一个版本](/docs/start/publish)：马上发一个版本
- [部署与运维](/docs/server/deploy)：systemd、反向代理、备份与升级
- [服务端 API](/docs/api)：接口总览与错误语义
