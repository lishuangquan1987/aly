# Aly 项目规则

> 所有新增/修改代码必须遵循以下规则。

---

## 一、命名规则（publish-gui-csharp — 历史实现）

### 1.1 文件命名

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

### 1.2 代码命名

| 元素 | 规则 | 示例 |
| ---- | ---- | ---- |
| 命名空间 | `AlyPublish.{Layer}` | `AlyPublish.Services` |
| 类 | PascalCase | `MainWindowViewModel` |
| 接口 | `I` 前缀 + PascalCase | `IProjectApi` |
| 属性 | PascalCase | `ServerUrl` |
| 字段（private） | `_camelCase` | `_configService` |
| 方法参数 / 局部变量 | camelCase | `serverUrl`, `filteredProjects` |
| 常量 | PascalCase | `DefaultPort` |

---

## 二、模型设计规则（publish-gui-csharp — 历史实现）

### 2.1 DTO（`AlyPublish.Models.Cli`）

- 用于 API 序列化/反序列化，使用 Newtonsoft.Json `[JsonProperty]`
- 属性 `{ get; set; }`，集合默认 `new()`，字符串默认 `string.Empty`

### 2.2 本地模型（`AlyPublish.Models.Local`）

- 用于 UI 绑定，继承 `ObservableObject`，使用 `[ObservableProperty]`
- 枚举类型定义在同文件中

---

## 三、UI 组件设计原则（publish-gui-csharp 历史实现；Go 版遵循 MyGo 控件原则，见 §八）

- **能用系统/库自带控件的，绝不自定义 UserControl**
- 自定义只用于组合多个控件形成有意义的业务单元
- 避免为了一行代码就创建一个 UserControl

---

## 四、禁止事项

1. ❌ 不在 XAML code-behind 中写业务逻辑
2. ❌ 不硬编码服务器地址
3. ❌ 不引用未使用的命名空间
4. ❌ 不在生产代码中写 `Console.WriteLine`（用 Serilog）
5. ❌ 不为了简单控件创建自定义 UserControl
6. ❌ 写出不兼容的代码。client为go1.10编写(GOPATH模式, GOARCH=386)，需要兼容xp，这个必须遵守。源码位于 client/aly-client/
7. ❌ 不建议、不擅自为 server API 增加鉴权/token/速率限制等机制。**API 无鉴权是有意为之的简单设计**（定位为可信内网工具），不为此增加复杂度；审查/评审时不得将"无鉴权"本身判为缺陷。
8. ❌ 不建议、不擅自引入"发布端删除同步"（本地删文件 → push 删服务端文件）。**客户端删除的文件服务端不删除是有意为之的简单设计**（2026-11-23 d17cc69 移除该功能：防误删只存在于服务端的资源；服务端文件永久保留，`file/delete_file` 接口已下线）。客户端的文件删除仍按版本清单差集在客户端本地执行（上一版清单 ∖ 本版清单），与发布端无关；审查/评审时不得将"服务端保留已删除文件"本身判为缺陷。

---

## 五、代码审查与提交

**每次涉及代码改动，必须执行以下流程：**

1. 写代码前如有不清楚的地方，主动提问，不要猜测或假设

2. 任务完成后，先编译，然后对本次改动进行代码审查

3. 修复审查中发现的问题

4. 审查通过后，提交代码（`git add` + `git commit`）

**不允许跳过审查直接提交**。

---

## 六、编码警告（PowerShell / 批处理）

### 6.1 PowerShell 写文件

> PowerShell `Set-Content` 不加 `-Encoding UTF8` 会将 UTF-8 中文按 GBK 编码导致乱码。
>
> **编辑含中文的文件时，始终使用 Python：**
> `python -c "with open(path,'r',encoding='utf-8') as f: c=f.read(); ..."`
>
> **禁止**：`Set-Content`、`Out-File`、`echo >`（不加 `-Encoding UTF8`）
> **已乱码时**：`git checkout -- <file>` 恢复后用 Python 重新编辑

### 6.2 批处理文件（`*.bat` / `*.cmd`）必须纯 ASCII + CRLF

> cmd.exe 按 OEM 代码页（中文 Windows 为 936/GBK）逐字节解析批处理。文件里出现
> UTF-8 中文会让 cmd 的行偏移算错：它把注释行的碎片当命令执行（报
> `'xxx' 不是内部或外部命令`），并把 `set` 行劈成两半——`GOARCH`/`GOROOT`
> 静默没设上，紧接着报"系统找不到指定的路径"。行尾被重复转换成 `\r\r\n`
> 同样有毒：每条 `set` 的值都会带一个尾随 CR。
>
> - 注释、`echo` 一律用英文；中文说明写到 `*.md` 里（如 `client/aly-client/BUILD.md`）。
> - `.bat` 里若要跑会输出中文的 Go 程序（`deploy-client.go` / `deploy-server.go`），
>   先 `chcp 65001 >nul`，否则控制台按 936 显示成乱码。
> - 自检（应无任何输出）：`git grep -nP '[^\x00-\x7F]' -- '*.bat' '*.cmd'`
> - 不硬编码 `GOROOT`/`GOPATH` 的机器路径：清空 `GOROOT` 让 go 从自身可执行文件推导，
>   `GOPATH` 未定义时回落到 `%USERPROFILE%\go`。

---

## 七、文档参考

- **Avalonia 12.1**: https://docs.avaloniaui.net/docs/welcome
- **Semi.Avalonia 12.1**: https://github.com/irihi/Semi.Avalonia
- **Ursa.Avalonia 2.2**: https://github.com/irihi/Ursa

---

## 八、MyGo 主实现（publish/publish-gui-go）

- `publish/publish-gui-go` 是发布 GUI 的**主实现**（文档、网站与 Release Build 默认都指向它）；
  `publish-gui-csharp` 为历史实现，仅保留源码，不再默认构建发布。
- 使用 MyGo（`github.com/egoist/mygo`）开发、修改或审查 `publish/publish-gui-go/` 时，**必须先加载技能**：
  `C:\Users\5001494\.agents\skills\mygo`（DeepSeek Harness 中对应技能名 `mygo`）。
- 技能内的 `reference/architecture.md`、`reference/views.md`、`reference/controls/*.md` 是 native UI
  的权威参考；API 以 `go.mod` 锁定的 MyGo 版本（当前 v0.8.3）源码为准，技能按更新版本编写时注意核对差异。
- 约束：Go 1.27.3；纯 Go、无 cgo；只用 native UI（`ui` 包），不使用 WebView/HTML/JS。
- 本模块遵循 Go 规范，不套用上面的 C#/XAML 命名规则；构建、测试与发布口径见
  `publish/publish-gui-go/README.md` 与根 `README.md`。

> **版本**: 2.3 | **适用范围**: publish/publish-gui-go/（MyGo 主实现，见 §八）与 publish/publish-gui-csharp/（C#/Avalonia 历史实现，见 §一/§二）
