# aly-publish 发布工具

`aly-publish` 是发布侧的命令行工具（cobra 架构），负责项目配置管理、文件差异扫描、暂存与推送。桌面版 `aly-publish-gui`（Avalonia）包装同样的能力，面向非技术发布人员。

## 命令树

```text
aly-publish
├── config            # 项目配置管理（.updator/ 目录）
│   ├── init          #   初始化配置（--server --project）
│   ├── set           #   设置单项配置
│   ├── set-array     #   设置数组配置（如忽略规则）
│   ├── get           #   读取单项配置
│   ├── list          #   列出全部配置
│   └── path          #   显示配置文件路径
├── project           # 服务端项目生命周期
│   ├── list          #   列出服务端所有项目
│   ├── create        #   创建项目
│   ├── update        #   更新项目配置
│   ├── delete        #   删除项目
│   └── info          #   项目详情
├── status            # 本地与服务端的文件差异总览
├── diff              # 逐文件详细对比（新增/修改/删除 + 哈希）
├── add               # 添加变更到暂存区（--all 或指定路径）
├── reset             # 从暂存区移除
├── staged            # 查看暂存区内容
├── push              # 推送暂存文件并发布版本（--version --message）
├── log               # 查看版本变更日志
└── server info       # 服务端系统信息
```

## 典型工作流

```bash
# 一次性：进入发布目录，初始化
cd ./dist
aly-publish config init --server http://10.0.0.1:2000 --project myapp

# 设置忽略规则（随版本分发，客户端应用更新时同样遵守）
aly-publish config set-array ignore_folders logs temp
aly-publish config set-array ignore_files "*.log" "*.tmp"

# 日常发版三步
aly-publish status           # 看这次改了什么
aly-publish add --all        # 暂存全部变更
aly-publish push --version V1.0.2 --message "修复登录超时问题"
```

## 关键行为

- **差异扫描**：`status` / `diff` 以服务端最新清单为基准，用 MD5 + SHA256 比对，输出新增 / 修改 / 删除三类变更。
- **分片上传**：超过阈值的文件自动走 `upload_chunk` → `upload_chunk_complete` 分片链路，网络中断后可续传。
- **忽略规则**：`ignore_folders` / `ignore_files` 在**扫描**（决定上传什么）与客户端**应用**（决定拷贝时跳过什么）两端同时生效。
- **版本单调**：`push` 使用的版本号必须严格大于服务端当前最新版本，否则服务端拒绝——要回滚内容，请发布更高的版本号。
- **发布 GUI**：`publish-gui` 是 Avalonia 桌面程序，与 CLI 共享 `.updator/` 配置与暂存区，二者可混用。
