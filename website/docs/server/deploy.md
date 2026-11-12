---
title: 部署 aly-server
description: aly-server 的单二进制部署：启动参数、数据目录布局、SQLite WAL 与自动迁移、systemd 单元、反向代理与安全加固、备份恢复与升级。
---

# 部署 aly-server

aly-server 是更新系统的服务端：保存项目与版本元数据、接收发布产物、向更新器提供文件清单与下载。

**部署形态**：单个静态编译的二进制，内嵌 SQLite 驱动（`modernc.org/sqlite`，纯 Go 实现），不依赖 CGO，不需要额外的数据库进程或运行时。除二进制自身写出的 `data/` 目录与 `aly.db` 外没有外部依赖。

## 获取二进制

发布产物按 `aly-server-{goos}-{goarch}` 命名，下表由最新 Release 生成，点击文件名即可下载（每次发版自动更新）：

<ReleaseDownloads group="server" />

也可以从源码构建（产物是纯静态二进制，`CGO_ENABLED=0`）：

```bash
cd server
CGO_ENABLED=0 go build -ldflags="-s -w" -o aly-server .
```

## 启动与命令行参数

服务端只有两个参数：

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-p` | int | `2000` | 监听端口 |
| `-db` | string | 空 | 数据库文件路径；为空时使用可执行文件目录下的 `aly.db` |

```bash
# 默认：监听 2000，数据库与数据目录都在可执行文件旁边
./aly-server

# 指定端口与数据库位置
./aly-server -p 8080 -db /var/lib/aly/aly.db
```

启动后输出 `启动中,监听端口:2000`，随后是 Gin 的访问日志。进程收到 `SIGINT` / `SIGTERM` 后优雅关闭：停止接收新请求，最多等待 10 秒完成在途请求，再关闭数据库。

监听地址是 `0.0.0.0:<port>`，没有只监听回环地址的开关。只允许本机访问时应使用防火墙限制，或让反向代理监听对外端口。

## 目录布局

```text
/opt/aly/
├── aly-server                    # 二进制
├── aly.db                        # SQLite 主库
├── aly.db-wal                    # WAL 日志（运行时存在）
├── aly.db-shm                    # 共享内存索引（运行时存在）
└── data/
    ├── demo/                     # 项目名 → 目录名
    │   ├── app.exe
    │   ├── config/
    │   │   └── app.json
    │   └── .updator_manifest.json  # 最近一次发布固化的清单
    └── demo2/
```

<ScreenshotFrame src="/images/server-layout-zh.svg" alt="aly-server 目录布局：二进制、SQLite 主库和 data/ 项目目录位于同一层" caption="data/ 与 aly.db 固定在可执行文件旁边，每个项目一个目录" wide />

关键点：

- **`data/` 固定在可执行文件所在目录**，由 `os.Executable()` 推导，与启动时的工作目录无关。`systemd` 的 `WorkingDirectory` 不影响它。
- 数据库默认也在可执行文件目录；只有用 `go run` 运行（可执行文件落在临时目录）时才回退到当前工作目录。
- `data/{projectName}` 由服务端按需创建：创建项目、首次上传文件都会建目录。
- `.updator_manifest.json` 是更新协议文件，不会出现在 `get_all_files` 与发布清单里。
- 上传中间态 `{文件}.chunks/`（分片暂存）与 `{文件}.merging`（合并临时文件）同样被过滤，只在传输过程中短暂存在。

## 数据库：SQLite + WAL + 自动迁移

服务端启动时按以下顺序初始化数据库（`server/internal/db/db.go`）：

1. 解析库路径：`-db` 优先，否则 `可执行文件目录/aly.db`。
2. `MkdirAll` 创建库文件所在目录（权限 `0700`）。
3. 连接串附加 `_fk=1`，并执行 `PRAGMA foreign_keys = ON`。
4. `PRAGMA journal_mode=WAL`：读写并发不互相阻塞，崩溃后可恢复。
5. `PRAGMA busy_timeout=5000`：锁冲突时最多重试 5 秒，避免直接报 `database is locked`。
6. `SetMaxOpenConns(1)`：SQLite 单写入者，连接池限为 1，从根上规避多连接下的锁升级。
7. `Client.Schema.Create`：ent 自动迁移，**启动即建表/加列**，无需手工执行 SQL。
8. 一次性数据迁移：老库 `project_change_logs` 表缺少 `project_id` 列时，先 `ADD COLUMN project_id`，再把旧列 `project_change_logs` 的值搬过去，最后 `DROP COLUMN` 删除旧列（删除失败只记日志，不影响启动）。

因此：

- 升级二进制不需要单独的迁移步骤，首次启动会自动完成；
- **不要**用两个进程同时打开同一个库文件（例如重复启动两份服务）；
- 备份时必须把 `aly.db`、`aly.db-wal`、`aly.db-shm` 视为一个整体，或先停服务让 WAL 落盘。

WAL 模式会产生 `aly.db-wal` / `aly.db-shm`，这是正常现象，不要手工删除。

## systemd 部署（Linux）

准备目录与账号（数据库和数据目录都需要写权限，因此放在同一个可写目录下）：

```bash
sudo useradd --system --home /opt/aly --shell /usr/sbin/nologin aly
sudo mkdir -p /opt/aly
sudo install -m 0755 aly-server-linux-amd64 /opt/aly/aly-server
sudo chown -R aly:aly /opt/aly
```

`/etc/systemd/system/aly-server.service`：

```ini
[Unit]
Description=Aly update server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=aly
Group=aly
WorkingDirectory=/opt/aly
ExecStart=/opt/aly/aly-server -p 2000
Restart=on-failure
RestartSec=3

# 优雅关闭：SIGTERM 触发 10 秒收尾，留出余量
KillSignal=SIGTERM
TimeoutStopSec=15

# 数据目录与 aly.db 都在 /opt/aly，未使用临时目录，可安全启用
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ReadWritePaths=/opt/aly

StandardOutput=journal
StandardError=journal
SyslogIdentifier=aly-server

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now aly-server
sudo systemctl status aly-server
sudo journalctl -u aly-server -f
```

注意：

- `ProtectSystem=full` 只把 `/usr`、`/boot`、`/etc` 挂成只读，`/opt/aly` 仍可写；`ReadWritePaths=/opt/aly` 在这里是显式声明。若改用更严格的 `ProtectSystem=strict`，这一行就是必需的，否则数据库与上传都会失败；
- 访问日志走 stdout、运行日志（缓存命中、清单写入失败等）走 stderr，两者都已导入 journal；
- 若把二进制放在 `/usr/local/bin`，`data/` 与 `aly.db` 会落到 `/usr/local/bin`，通常不可写 —— 用 `-db` 只解决数据库，`data/` 无法改位置，所以建议二进制单独放一个可写目录。

### Windows

直接运行 `aly-server-windows-amd64.exe -p 2000`，`data\` 与 `aly.db` 生成在 exe 同目录。注册为 Windows 服务可用 NSSM 等第三方包装器；仓库未提供安装脚本。

## 反向代理

服务端只提供 HTTP。对外暴露时在前面放一层 Nginx（或同类代理）做 TLS 终止。

```nginx
server {
    listen 443 ssl;
    server_name update.example.com;

    ssl_certificate     /etc/nginx/ssl/update.example.com.crt;
    ssl_certificate_key /etc/nginx/ssl/update.example.com.key;

    # 服务端自身限制 1 GB；这里略高一点，让超限由服务端返回 JSON 错误而不是代理的 413
    client_max_body_size 1050m;

    # 上传/下载可能持续数分钟，避免代理先超时
    proxy_connect_timeout 10s;
    proxy_send_timeout    600s;
    proxy_read_timeout    600s;

    # 大文件不再由 Nginx 先完整落盘
    proxy_request_buffering off;

    location / {
        proxy_pass http://127.0.0.1:2000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;

        # 下载是二进制流，不要压缩、不要改写响应体
        gzip off;
    }
}
```

三个容易踩的点：

1. **不要挂在路径前缀下**。客户端的 `buildAPIURL` 只取 `serverURL` 的 `scheme://host`，路径部分会被丢弃；配成 `https://example.com/aly` 实际请求的是 `https://example.com/api/...`。用独立域名或独立端口承载。
2. **不要加认证层**。现有 aly-client 与 aly-publish 都不发送 `Authorization` 头，也不支持客户端证书，代理层加 Basic Auth / Token 会让官方客户端全部 401。需要跨网访问时用防火墙 IP 白名单或 VPN。
3. **保留 Range 语义**。`download_file` 用 `http.ServeContent` 实现断点续传；代理不要缓存完整响应或改写 `206` / `Content-Range`，否则大文件续传会退化为整文件重下。

TLS 本身是支持的：`serverURL` 直接写 `https://update.example.com` 即可。

## 安全加固

| 措施 | 说明 |
| --- | --- |
| 网络隔离 | 只在内网/办公网开放端口，公网入口仅经代理 |
| 防火墙 | 只放通代理所在主机到 `2000`，其余拒绝 |
| 运行账号 | 用独立低权限账号，`/opt/aly` 属主为该账号，权限 `0750` 以内 |
| 文件权限 | `data/` 不含机密，但含可执行文件；禁止 Web 服务器把该目录当静态站点暴露 |
| 备份加密 | 备份包含全部发布产物，按内部资产对待 |
| 审计 | 保留 `journalctl` 访问日志；服务端无内置审计与限流 |

服务端不校验项目名以外的输入语义（例如发布脚本内容），`after_apply_update_script` 只是被存储与下发，执行发生在客户端，因此发布权限等同于对终端机的代码执行权限。

## 备份与恢复

需要备份的只有两样：`aly.db`（元数据）与 `data/`（发布产物）。

### 停机全量备份（推荐）

```bash
sudo systemctl stop aly-server
sudo tar -czf /backup/aly-$(date +%F-%H%M).tar.gz -C /opt/aly aly.db data
sudo systemctl start aly-server
```

停机后 WAL 已随数据库关闭而落盘，`aly.db` 是完整一致的；`data/` 与库内容也处于同一时刻。

### 运行中备份

数据库可用 SQLite 的在线备份：

```bash
sqlite3 /opt/aly/aly.db ".backup '/backup/aly-$(date +%F).db'"
```

但 `data/` 仍需单独同步（如 `rsync -a`）。数据库与文件目录不是原子快照：极端情况下会备份到「日志已登记版本、文件尚未拷完」的状态 —— 客户端可能拿到不完整版本。生产环境优先停机备份。

### 恢复

```bash
sudo systemctl stop aly-server
sudo rm -f /opt/aly/aly.db /opt/aly/aly.db-wal /opt/aly/aly.db-shm
sudo tar -xzf /backup/aly-2026-11-10-1200.tar.gz -C /opt/aly
sudo chown -R aly:aly /opt/aly
sudo systemctl start aly-server
```

验证：

```bash
curl -s http://127.0.0.1:2000/api/project/get_all_projects
curl -s http://127.0.0.1:2000/api/file/get_all_files/demo
```

第一条应列出项目，第二条应列出文件且带 `md5` / `sha256`。若项目在、文件列表为空，通常是 `data/` 没恢复或权限不对。

## 升级

```bash
sudo systemctl stop aly-server
sudo cp -a /opt/aly/aly.db /backup/aly.db.bak
sudo cp /opt/aly/aly-server /opt/aly/aly-server.bak
sudo install -m 0755 aly-server-linux-amd64 /opt/aly/aly-server
sudo systemctl start aly-server
```

- 不需要手工迁移：建表与加列在启动时由 ent 自动完成，老库的 `project_id` 列迁移也在启动时执行。
- 升级前备份 `aly.db`：schema 迁移只向前，新版本可能已加列或改列。
- **回退**：换回旧二进制即可，但如果新版本已经执行过 schema 变更，保守做法是同时恢复升级前的 `aly.db` 备份；只换二进制不保证旧版本能读新库。
- `data/` 不随版本变化，无需处理；`.updator_manifest.json` 会在下次发布时重写。

## 排障

| 现象 | 常见原因 | 处理 |
| --- | --- | --- |
| 启动即退出，日志 `failed opening connection to sqlite` | `-db` 目录不可写 | 检查目录属主与权限 |
| 启动即退出，日志 `failed creating schema resources` | 库文件损坏或被其他进程占用 | 停掉重复实例；用备份恢复 |
| 上传返回「文件过大」 | 单文件超 1 GB，或代理 `client_max_body_size` 太小 | 调整代理限制；确认是业务需要 |
| 代理返回 413 | Nginx 默认 1 MB 限制 | 设 `client_max_body_size 1050m` |
| 下载 404 | `path` 不是 `data/` 下的绝对路径，或二进制搬过位置 | 用 `get_all_files` 的 `fileAbsolutePath` 原样请求 |
| 下载到一半中断 | 服务端 `WriteTimeout` 为 60 秒（整次响应上限） | 慢链路大文件会被截断；客户端按 Range 续传重试 |
| `database is locked` | 同一库文件被多进程打开 | 保证只有一个 aly-server 实例 |
| 变更日志时间与本地不一致 | `time` 由发布端传入字符串，服务端原样存储 | 发布端统一使用 `yyyy-MM-dd HH:mm:ss` 与同一时区 |

服务端没有内置的版本查询接口，确认运行版本以 `GET /api/server/info` 返回的 `version`（Go 运行时版本）与二进制文件名/发布记录为准。

## 相关页面

- [API 总览](/docs/api)：信封、错误语义、接口索引
- [项目接口](/docs/server/api-project) / [文件接口](/docs/server/api-file)：接口级细节
- [数据模型](/docs/server/api-models)：库表与 JSON 字段
- [发布工具 aly-publish](/docs/publish)：向服务端推送文件与版本的一方