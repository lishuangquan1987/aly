# Aly 项目规则

> 所有新增/修改代码必须遵循以下规则。

---

## 一、命名规则（publish-gui-csharp）

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

## 二、模型设计规则（publish-gui-csharp）

### 2.1 DTO（`AlyPublish.Models.Cli`）

- 用于 API 序列化/反序列化，使用 Newtonsoft.Json `[JsonProperty]`
- 属性 `{ get; set; }`，集合默认 `new()`，字符串默认 `string.Empty`

### 2.2 本地模型（`AlyPublish.Models.Local`）

- 用于 UI 绑定，继承 `ObservableObject`，使用 `[ObservableProperty]`
- 枚举类型定义在同文件中

---

## 三、UI 组件设计原则

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

---

## 五、代码审查与提交

**每次涉及代码改动，必须执行以下流程：**

1. 写代码前如有不清楚的地方，主动提问，不要猜测或假设

2. 任务完成后，先编译，然后对本次改动进行代码审查

3. 修复审查中发现的问题

4. 审查通过后，提交代码（`git add` + `git commit`）

**不允许跳过审查直接提交**。

---

## 六、PowerShell 编码警告

> PowerShell `Set-Content` 不加 `-Encoding UTF8` 会将 UTF-8 中文按 GBK 编码导致乱码。
>
> **编辑含中文的文件时，始终使用 Python：**
> `python -c "with open(path,'r',encoding='utf-8') as f: c=f.read(); ..."`
>
> **禁止**：`Set-Content`、`Out-File`、`echo >`（不加 `-Encoding UTF8`）
> **已乱码时**：`git checkout -- <file>` 恢复后用 Python 重新编辑

---

## 七、文档参考

- **Avalonia 12.1**: https://docs.avaloniaui.net/docs/welcome
- **Semi.Avalonia 12.1**: https://github.com/irihi/Semi.Avalonia
- **Ursa.Avalonia 2.2**: https://github.com/irihi/Ursa

---

## 八、MyGo 实验模块（publish/publish-gui-go）

- 使用 MyGo（`github.com/egoist/mygo`）开发、修改或审查 `publish/publish-gui-go/` 时，**必须先加载技能**：
  `C:\Users\5001494\.agents\skills\mygo`（DeepSeek Harness 中对应技能名 `mygo`）。
- 技能内的 `reference/architecture.md`、`reference/views.md`、`reference/controls/*.md` 是 native UI
  的权威参考；API 以 `go.mod` 锁定的 MyGo 版本（当前 v0.8.3）源码为准，技能按更新版本编写时注意核对差异。
- 约束：Go 1.27.3；纯 Go、无 cgo；只用 native UI（`ui` 包），不使用 WebView/HTML/JS。
- 本模块遵循 Go 规范，不套用上面的 C#/XAML 命名规则；它是实验项目，稳定前不同步到网站/根 README。

> **版本**: 2.2 | **适用范围**: publish/publish-gui-csharp/（C#/Avalonia）与 publish/publish-gui-go/（MyGo 实验；Go 规范，见 §八）
