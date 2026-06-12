# Zap 项目规则

> 所有新增/修改代码必须遵循以下规则。

---

## 一、命名规则（publish-gui）

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
| 命名空间 | `ZapPublish.{Layer}` | `ZapPublish.Services` |
| 类 | PascalCase | `MainWindowViewModel` |
| 接口 | `I` 前缀 + PascalCase | `IProjectApi` |
| 属性 | PascalCase | `ServerUrl` |
| 字段（private） | `_camelCase` | `_configService` |
| 方法参数 / 局部变量 | camelCase | `serverUrl`, `filteredProjects` |
| 常量 | PascalCase | `DefaultPort` |

---

## 二、模型设计规则（publish-gui）

### 2.1 DTO（`ZapPublish.Models.Cli`）

- 用于 API 序列化/反序列化，使用 Newtonsoft.Json `[JsonProperty]`
- 属性 `{ get; set; }`，集合默认 `new()`，字符串默认 `string.Empty`

### 2.2 本地模型（`ZapPublish.Models.Local`）

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
5. ❌ 不硬编码服务器地址
8. ❌ 不引用未使用的命名空间
9. ❌ 不在生产代码中写 `Console.WriteLine`（用 Serilog）
10. ❌ 不为了简单控件创建自定义 UserControl
11.❌写出不兼容的代码。client为go1.10编写，需要兼容xp,这个必须遵守

---

## 五、代码审查与提交

**每次涉及代码改动，必须执行以下流程：**

1. 写代码前如有不清楚的地方，主动提问，不要猜测或假设

2. 任务完成后，先编译，然后使用 `open-code-review` 技能对本次改动进行代码审查

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

- **Avalonia 12**: https://docs.avaloniaui.net/docs/welcome
- **Semi.Avalonia**: https://github.com/irihi/Semi.Avalonia
- **Ursa.Avalonia**: https://github.com/irihi/Ursa

> **版本**: 2.0 | **适用范围**: publish/publish-gui/（.NET 8 + Avalonia 12 + CommunityToolkit.Mvvm + Semi.Avalonia + Ursa.Avalonia）
