# ClientUpdator 项目规则

> 所有新增/修改代码必须遵循以下规则。

---

## 〇、项目概述与职责边界

| 子项目 | 目录 | 职责 | 技术栈 |
| ------ | ---- | ---- | ------ |
| **server** | `server/` | 服务端，提供项目管理和文件管理 REST API | Go 1.25 + Gin + Ent + SQLite |
| **publish-gui** | `publish/publish-gui/` | 版本发布 GUI 工具，管理项目、上传文件、发布版本 | .NET 8 + Avalonia 12 + CommunityToolkit.Mvvm |
| **publish-cli** | `publish/publish-cli/` | 版本发布 CLI 工具，命令行发布管理，类 git 工作流 | Go 1.25 + cobra |
| **client** | `client/` | 终端用户更新程序，检查更新、下载更新、应用更新 | Go 1.10（兼容 Windows XP） |

**关键约束**：
- `publish-gui` / `publish-cli` 面向**发布者**，负责上传文件到服务端、管理版本、触发发布
- `client` 面向**终端用户**，负责从服务端检测新版本、下载更新包、执行文件替换
- `server` 是三端共同依赖的后端，API 路径和数据结构必须与三端保持一致
- 修改 `server` API 时，必须同步确认 GUI/CLI 的 DTO 和 Service 层是否需要调整
- 开发 `client` 时，应复用 `server` 已有 API 端点，避免单独新增接口（除非必要）

---

## 一、项目结构

```
ClientUpdator/
├── server/                            # Go 服务端
│   ├── cmd/main.go                    # 入口
│   ├── cmd/gen/main.go                # ent 代码生成入口
│   ├── controllers/                   # HTTP 控制器
│   ├── ent/                           # ent ORM 生成代码
│   ├── internal/                      # 内部包（db, service, utils）
│   ├── models/                        # 数据模型
│   └── routers/routers.go             # 路由定义
│
├── client/                            # Go 客户端（终端用户更新）
│   ├── main.go                        # 入口
│   ├── client.yaml                    # 运行时配置
│   ├── client/http_client.go          # HTTP 客户端
│   ├── cmd/                           # cobra 命令（check_update, download_update, apply_update, rollback 等）
│   ├── config/                        # 配置与版本管理
│   ├── model/dto.go                   # 数据传输对象
│   └── util/                          # 文件操作、进程管理工具
│
├── publish/
│   ├── publish-gui/                   # C# GUI 发布工具
│   │   ├── PublishGui.slnx
│   │   └── src/PublishGui/
│   │       ├── App.axaml(.cs)         # 应用入口 + 主题初始化
│   │       ├── Program.cs             # 启动入口
│   │       ├── ViewLocator.cs         # ViewModel 自动解析
│   │       ├── Constants/             # 常量定义
│   │       ├── Converters/            # IValueConverter 实现
│   │       ├── Helpers/               # 静态工具类
│   │       ├── Models/
│   │       │   ├── Cli/               # CLI 输出 DTO（与服务端 JSON 对应）
│   │       │   └── Local/             # 本地业务模型（ObservableObject 子类）
│   │       ├── Services/              # 服务层（CliService, ConfigService, ProcessService）
│   │       ├── ViewModels/            # ViewModel 层
│   │       └── Views/
│   │           ├── MainWindow.axaml   # 主窗口
│   │           └── Dialogs/           # 对话框
│   │
│   └── publish-cli/                   # Go CLI 发布工具
│       ├── cmd/publish-cli/main.go    # 入口
│       ├── internal/
│       │   ├── api/client.go          # HTTP 客户端
│       │   ├── cmd/                   # cobra 命令（add, push, status, log, config, watch 等）
│       │   ├── config/config.go       # 配置管理
│       │   ├── diff/scanner.go        # 文件扫描 + 差异比对
│       │   └── staging/staging.go     # 暂存区管理
│       └── pkg/models/models.go       # DTO 数据模型
│
└── README.md
```

**分层依赖**：View → ViewModel → Service → Model（单向，禁止反向引用）

---

## 二、命名规则（publish-gui）

### 2.1 文件命名

| 类型 | 规则 | 示例 |
| ---- | ---- | ---- |
| DTO（Models/Cli/） | `{EntityName}.cs` | `CliOutput.cs`, `ProjectInfo.cs` |
| 本地模型（Models/Local/） | `{EntityName}.cs` | `ProjectConfig.cs`, `FileItem.cs` |
| Service | `{Domain}Service.cs` | `CliService.cs`, `ConfigService.cs` |
| ViewModel | `{Page}ViewModel.cs` | `MainWindowViewModel.cs` |
| View | `{Name}.axaml + .axaml.cs` | `MainWindow.axaml` |
| Dialog | `{Action}Dialog.axaml + .axaml.cs` | `AddProjectDialog.axaml` |
| Converter | `{Source}To{Target}Converter.cs` | `BytesToSizeConverter.cs` |
| Helper | `{Function}Helper.cs` | `FileSizeHelper.cs` |

### 2.2 代码命名

| 元素 | 规则 | 示例 |
| ---- | ---- | ---- |
| 命名空间 | `PublishGui.{Layer}` | `PublishGui.Services` |
| 类 | PascalCase | `MainWindowViewModel` |
| 接口 | `I` 前缀 + PascalCase | `IProjectApi` |
| 属性 | PascalCase | `ServerUrl` |
| 字段（private） | `_camelCase` | `_configService` |
| 方法参数 / 局部变量 | camelCase | `serverUrl`, `filteredProjects` |
| 常量 | PascalCase | `DefaultPort` |

### 2.3 XAML 命名

- `x:Name`：PascalCase（如 `ProjectListBox`）
- 绑定路径：PascalCase（如 `{Binding ServerUrl}`）
- 资源键：PascalCase（如 `{DynamicResource WindowBackgroundBrush}`）

---

## 三、MVVM 架构规则（publish-gui）

### 3.1 ViewModel 规范

继承 `ObservableObject`，使用 `partial class` + 源生成器：

```csharp
public partial class MyViewModel : ObservableObject
{
    [ObservableProperty]
    private string _someProperty = string.Empty;

    partial void OnSomePropertyChanged(string value) { /* 副作用 */ }

    [RelayCommand]
    private async Task DoSomething() { /* 命令实现 */ }
}
```

- `[ObservableProperty]` 字段 `_camelCase`，必须提供初始值
- 集合类型使用 `ObservableCollection<T>`，可空引用标注 `?`
- 异步命令 `async Task`，同步命令 `void`，通过 `{Name}Command.ExecuteAsync(null)` 调用

### 3.2 View 绑定

- code-behind 构造函数中通过 DI 设置 DataContext（或通过 ViewLocator 自动解析）
- XAML 中不写业务逻辑，code-behind 只处理纯 UI 事件

---

## 四、服务层规则（publish-gui）

### 4.1 HTTP 服务

使用 `Flurl.Http`，API 返回强类型 DTO，通过 `CommonResponse<T>` 解包，方法命名 `{Action}{Target}Async`：

```csharp
public async Task<List<ProjectDto>> GetAllProjectsAsync(string serverUrl)
{
    var response = await $"{serverUrl}/api/project/get_all_projects"
        .GetJsonAsync<CommonResponse<List<ProjectDto>>>();
    return response.Data ?? new List<ProjectDto>();
}
```

### 4.2 本地服务

- 文件路径用 `Path.Combine`，配置持久化用 JSON（`{LocalApplicationData}/PublishGui/`）
- 日志用 Serilog（`{LocalApplicationData}/PublishGui/logs/log.log`）
- 文件操作在 Service 层封装，ViewModel 不直接使用 `System.IO`

### 4.3 服务注册

- Singleton：全局无状态服务（ConfigService、CliService、ProcessService）
- Transient：ViewModel
- 使用 `Microsoft.Extensions.DependencyInjection`

---

## 五、API 兼容性规则（三端统一）

### 5.1 JSON 字段命名

DTO 属性必须使用 `[JsonProperty("originalName")]` 标注。

**注意**：Go ent 生成的 JSON tag 是 `snake_case`（如 `force_update`），控制器请求体是 `camelCase`（如 `isForceUpdate`），两者不一致，必须逐一核对。

**统一约定**：三端外层包装（`isSuccess`/`errorMsg`）统一使用 **camelCase**。

```csharp
// GET 响应（ent → snake_case）
[JsonProperty("force_update")]
public bool IsForceUpdate { get; set; }

// POST 请求（控制器 → camelCase）
[JsonProperty("isForceUpdate")]
public bool IsForceUpdate { get; set; }
```

### 5.2 接口路径

| 方法 | 路径 | 用途 |
| ---- | ---- | ---- |
| GET | `/api/project/get_all_projects` | 获取所有项目 |
| POST | `/api/project/create_project` | 创建项目 + 初始版本 V1.0.0 |
| POST | `/api/project/update_project` | 更新项目配置 |
| POST | `/api/project/delete_project/{projectId}` | 软删除项目 |
| GET | `/api/project/get_project_change_logs/{projectId}` | 获取变更日志 |
| GET | `/api/project/get_project_os_info/{projectId}` | 获取服务端系统信息 |
| POST | `/api/project/publish_version` | 发布新版本 |
| POST | `/api/file/upload_file` | 上传文件（multipart） |
| GET | `/api/file/get_all_files/{projectId}` | 获取文件列表（含 MD5/SHA256） |
| GET | `/api/file/download_file?path=` | 下载文件 |

### 5.3 上传格式

```csharp
await $"{serverUrl}/api/file/upload_file"
    .PostMultipartAsync(mp => mp
        .AddFile("file", fileStream, fileName)
        .AddString("projectName", projectName)
        .AddString("relativeFileName", relativePath));
```

---

## 六、错误处理（publish-gui）

```csharp
[RelayCommand]
private async Task SomeOperation()
{
    IsOperationEnabled = false;
    StatusMessage = "正在执行...";
    try { /* 核心逻辑 */ StatusMessage = "操作成功"; }
    catch (FlurlHttpException ex) { StatusMessage = $"网络错误: {ex.Message}"; }
    catch (Exception ex) { StatusMessage = $"操作失败: {ex.Message}"; }
    finally { IsOperationEnabled = true; }
}
```

- 每个耗时操作必须有 `IsXxxEnabled` / `IsBusy` 控制 UI 状态，开始时立即禁用按钮
- 状态消息使用中文，异常必须记录到 Serilog，不能静默吞掉

---

## 七、XAML 规则（publish-gui）

### 7.1 布局与绑定

- Grid 布局，明确 RowDefinitions / ColumnDefinitions，Margin 用 4 的倍数
- `{Binding Property}` 不指定 Source，`Converter={StaticResource MyConverter}` 处理类型转换

### 7.2 Semi.Avalonia / Ursa.Avalonia 主题

- 窗口用 `<semi:Window>`（`xmlns:semi="https://irihi.tech/semi"`）
- 主题令牌：`SemiColorPrimary`、`SemiColorSuccess`、`SemiColorDanger`、`SemiColorText0` 等
- 按钮样式：`Classes="Primary"` / `"Tertiary"` / `"Danger"`
- 主题初始化在 `App.axaml`：`<semi:SemiTheme>` + `<semi:UrsaSemiTheme>`

---

## 八、模型设计规则（publish-gui）

### 8.1 DTO（`PublishGui.Models.Cli`）

- 用于 API 序列化/反序列化，使用 Newtonsoft.Json `[JsonProperty]`
- 属性 `{ get; set; }`，集合默认 `new()`，字符串默认 `string.Empty`

### 8.2 本地模型（`PublishGui.Models.Local`）

- 用于 UI 绑定，继承 `ObservableObject`，使用 `[ObservableProperty]`
- 枚举类型定义在同文件中

---

## 九、UI 组件设计原则

- **能用系统/库自带控件的，绝不自定义 UserControl**
- 自定义只用于组合多个控件形成有意义的业务单元
- 避免为了一行代码就创建一个 UserControl

---

## 十、禁止事项

1. ❌ 不在 XAML code-behind 中写业务逻辑
2. ❌ 不使用 `System.Windows.Forms`（Avalonia 不支持）
3. ❌ 不在 ViewModel 中引用 View 类型
4. ❌ 不用 `Task.Run` 处理 UI 线程（用异步命令 + `Dispatcher`）
5. ❌ 不硬编码服务器地址
6. ❌ 不写占位符 TODO
7. ❌ 不添加不必要的注释
8. ❌ 不引用未使用的命名空间
9. ❌ 不在生产代码中写 `Console.WriteLine`（用 Serilog）
10. ❌ 不为了简单控件创建自定义 UserControl

---

## 十一、代码审查与提交

**每次涉及代码改动，必须执行以下流程：**

1. 使用 `$open-code-review` 技能对本次改动进行代码审查
2. 修复审查中发现的问题
3. 审查通过后，提交代码（`git add` + `git commit`）

不允许跳过审查直接提交。

---

## 十二、交互与自检

- 写代码前如有不清楚的地方，主动提问，不要猜测或假设
- 将问题写在 `.trae/documents/Interactive.md` 中，等待回答后再继续
- 每次修改完成后，全面检查是否有 bug，确认任务是否完全完成

---

## 十三、PowerShell 编码警告

> PowerShell `Set-Content` 不加 `-Encoding UTF8` 会将 UTF-8 中文按 GBK 编码导致乱码。
>
> **编辑含中文的文件时，始终使用 Python：**
> `python -c "with open(path,'r',encoding='utf-8') as f: c=f.read(); ..."`
>
> **禁止**：`Set-Content`、`Out-File`、`echo >`（不加 `-Encoding UTF8`）
> **已乱码时**：`git checkout -- <file>` 恢复后用 Python 重新编辑

---

## 十四、文档参考

- **Avalonia 12**: https://docs.avaloniaui.net/docs/welcome
- **Semi.Avalonia**: https://github.com/irihi/Semi.Avalonia
- **Ursa.Avalonia**: https://github.com/irihi/Ursa

---

## 十五、各模块职责详解

### 整体架构

```
┌─────────────────────────────┐          ┌─────────────────────────────┐
│       发布者的工作站          │          │       终端用户的机器          │
│                             │          │                             │
│  ┌───────────┐              │          │                             │
│  │publish-gui│ 只存项目配置  │          │  ┌─────────┐               │
│  │ (GUI壳子) │ 不调用API    │          │  │ client  │──→ 应用程序    │
│  └─────┬─────┘              │          │  │ (更新器) │   (被更新目标) │
│        │ 子进程调用          │          │  └────┬────┘               │
│        ▼                    │          │       │                   │
│  ┌───────────┐              │          │       │                   │
│  │publish-cli│──────────────┼──┐       └───────┼───────────────────┘
│  │ (发布引擎) │              │  │               │
│  └─────┬─────┘              │  │               │
│        │ 扫描               │  │  HTTP         │  HTTP
│        ▼                    │  │               │
│  ┌───────────┐              │  │               │
│  │ 本地构建   │              │  │               │
│  │ 产物目录   │              │  │               │
│  └───────────┘              │  │               │
│                             │  │               │
└─────────────────────────────┘  │               │
                                 ▼               ▼
                          ┌───────────────────────────┐
                          │        server             │
                          │   项目管理 / 文件存储       │
                          │   版本记录 / 文件清单       │
                          └───────────────────────────┘
```

---

### server — 服务端

**定位：** 纯后端 API 服务，不包含任何业务逻辑，只负责数据存储和文件管理。

**核心职责：**
- **项目管理**：创建/更新/删除项目，记录项目名称、版本号、强制更新标志、忽略规则
- **文件管理**：接收上传文件、存储文件、提供文件下载（支持断点续传 Range 请求）、记录每个文件的 MD5/SHA256
- **版本管理**：记录每次发布的版本号、变更说明、发布时间，维护变更日志（ProjectChangeLog）
- **文件清单**：根据项目 ID 返回该版本下所有文件的路径、大小、校验值（供 client 比对差异）

**不做的事：**
- 不关心文件是怎么来的（CLI 传的还是 GUI 传的）
- 不关心谁在下载更新
- 不做文件差异计算（差异比对是 publish-cli 和 client 各自在本地完成的）

**关键数据模型：**
- `Project`：id, name, title, version, force_update, ignore_folders, ignore_files
- `ProjectChangeLog`：id, project_id, version, logs(数组), time
- 存储的文件按 `{exe_dir}/data/{project_name}/` 目录组织

---

### publish-cli — 发布引擎（核心）

**定位：** 类 Git 工作流的命令行工具，是整个发布流程的核心引擎。publish-gui 通过子进程调用它，不直接调用 server API。

**类比 Git 的核心概念：**

| Git 概念 | publish-cli 对应 | 存储位置 |
|----------|-----------------|---------|
| 工作区 (Working Directory) | 本地构建产物目录 | 开发者指定的 `--path` |
| 暂存区 (Staging Area / Index) | `staged-files.json` | `<projectPath>/.publish-cli/staging/staged-files.json` |
| 仓库 (Repository) | server 上的文件存储 | server API |
| `.git/config` | `.publish-cli/config.json` | `<projectPath>/.publish-cli/config.json` |
| `git add` | `publish-cli add` | 计算 MD5，写入暂存区 |
| `git status` | `publish-cli status` | 扫描本地文件 vs 服务端文件，显示差异 |
| `git push` | `publish-cli push` | 上传暂存区文件 + 创建版本记录 |

**配置层级（两级合并，项目覆盖全局）：**
- 全局配置：`~/.publish-cli/config.json`
- 项目配置：`<projectPath>/.publish-cli/config.json`

**配置结构：**
```json
{
  "server": { "url": "http://10.0.0.1:2000" },
  "project": { "name": "myapp", "path": "/path/to/build", "id": 1 },
  "ignore": { "folders": ["logs", "temp"], "files": ["*.log", ".DS_Store"] }
}
```

**核心命令：**

| 命令 | 作用 | 类比 |
|------|------|------|
| `config init` | 初始化项目配置（server地址、项目名、ID） | `git init` + `git remote add` |
| `status` | 扫描本地文件，对比服务端文件，显示新增/修改/删除/未变化 | `git status` |
| `add [--all \| <file>...]` | 将文件加入暂存区（计算 MD5 快照） | `git add` |
| `reset [--all \| <file>...]` | 将文件从暂存区移除 | `git reset` |
| `staged` | 查看暂存区内容 | `git diff --cached` |
| `push --version <v> --message <msg>` | 上传暂存区文件到 server + 创建版本记录 | `git push` |
| `push-all --version <v>` | 跳过暂存区，直接上传所有变更文件 | 快捷方式 |
| `publish --version <v> --message <msg>` | 一键流：status → add --all → push | `git add -A && git commit && git push` |
| `log [--limit N]` | 查看服务端版本历史 | `git log` |
| `watch [--auto-add]` | 轮询文件变化，可自动暂存 | 文件监控 |

**文件差异比对机制：**
1. `ScanDirectory()` 递归扫描本地目录（跳过 `.publish-cli` 和忽略规则），对每个文件同时计算 MD5 和 SHA256
2. 从 server 拉取文件清单（`GET /api/file/get_all_files/{projectId}`）
3. 按相对路径做 diff：本地有服务端没有 → `new`，MD5 不同 → `modified`，服务端有本地没有 → `deleted`

**发布流程（push）：**
1. 加载暂存区文件列表
2. `staging.Verify()` 重新计算 MD5，校验文件在 add 后是否被修改过
3. 逐个上传文件（`POST /api/file/upload_file`，multipart）
4. 调用 `POST /api/project/publish_version` 创建版本记录
5. 清空暂存区

---

### publish-gui — 发布 GUI

**定位：** 面向发布者的图形界面，**纯前端壳子**，所有发布操作都通过子进程调用 publish-cli 完成，自身不直接调用 server API。

**存储的内容（只存配置，不存数据）：**
- `%LOCALAPPDATA%/PublishGui/config.json`：项目列表（项目名、服务端地址、本地路径、项目 ID）
- 不缓存文件内容、不缓存版本历史，所有数据实时从 publish-cli 获取

**与 publish-cli 的关系：**
```
GUI 用户点击"刷新"  →  CliService.RunAsync("status")  →  ProcessService  →  publish-cli.exe status --json
GUI 用户点击"全部暂存" →  CliService.RunAsync("add --all") →  ProcessService  →  publish-cli.exe add --all --json
GUI 用户点击"发布"    →  CliService.RunAsync("push ...")  →  ProcessService  →  publish-cli.exe push --version ... --json
```

**唯一不经过 publish-cli 的操作：**
- 添加项目时调用 `publish-cli config init` 初始化本地配置
- 添加项目时通过 `publish-cli project list` / `publish-cli project create` 操作服务端项目

---

### client — 终端用户更新器

**定位：** 集成到最终用户的应用程序中，负责检测更新、下载更新、原子替换应用文件。用 Go 1.10 编译以兼容 Windows XP。

**更新状态机（version.json）：**
```
applied → (检测到新版本) → downloaded → applying → applied
                ↑                              │
                └──────── rollback ─────────────┘
```

**七个命令：**

| 命令 | 作用 | 使用场景 |
|------|------|---------|
| `check_update` | 比较本地版本与服务端最新版本，返回是否有更新及是否强制更新 | 应用启动时定期调用 |
| `check_diff` | 列出本地与服务端文件的 MD5/SHA256 差异 | 诊断更新问题 |
| `download_update` | 下载变更文件（仅下载 MD5 不同的文件），支持断点续传（>100MB 用 Range 请求） | 用户确认更新后 |
| `apply_update` | 原子文件夹替换（关闭进程 → 备份当前 → 替换为新版 → 重启应用） | 下载完成后 |
| `rollback --version X` | 回退到指定版本（版本目录是之前 apply 时保存的完整快照） | 新版有问题时 |
| `list_rollback_versions` | 列出可回退的版本目录 | 查看历史版本 |
| `check_self_update` | 检查更新器自身是否需要替换（比对 SHA256） | 更新器自身升级 |

**原子替换机制（apply_update）：**
1. 设置状态为 `applying`（写入 version.json）
2. 发送 `WM_CLOSE` 优雅关闭目标进程，超时后 `TerminateProcess`
3. 将当前应用目录备份为 `ApplicationFolder_{oldVersion}/`
4. 将新版目录重命名为应用目录
5. 设置状态为 `applied`，启动主程序
6. 如果中途崩溃，下次运行时根据 `applying` 状态自动恢复

**文件目录结构：**
```
PackageFolder/                          ← main_exe_relative_path 的父目录
├── ApplicationFolder/                  ← 当前活跃的应用目录（被更新的目标）
├── ApplicationFolder_V1.0.0/          ← 版本备份（可用于回退）
├── ApplicationFolder_V1.0.1/
└── UpdateFolder/                       ← client-updator.exe 所在目录
    ├── client-updator.exe
    ├── client.yaml
    ├── version.json
    └── logs/
```

---

### 模块间通信关系

```
publish-gui  ──子进程──→  publish-cli  ──HTTP──→  server  ←──HTTP──  client
   │                       │                       │                   │
   │ config.json           │ .publish-cli/         │ SQLite            │ client.yaml
   │ (项目名/路径/ID)       │ config.json           │ (项目/文件/版本)    │ version.json
   │                       │ staging/              │                   │ (版本/状态)
   │                       │ staged-files.json     │                   │
```

- **publish-gui → publish-cli**：通过 `ProcessService` 启动子进程，传入 `--json` 参数获取 JSON 输出
- **publish-cli → server**：HTTP REST API（上传文件、查询文件清单、创建版本）
- **client → server**：HTTP REST API（查询版本、获取文件清单、下载文件）
- **publish-gui ↔ publish-cli**：共享同一个 `.publish-cli/config.json`（GUI 通过 `config init` 写入，CLI 读取）

> **版本**: 2.0 | **适用范围**: publish/publish-gui/（.NET 8 + Avalonia 12 + CommunityToolkit.Mvvm + Semi.Avalonia + Ursa.Avalonia）
