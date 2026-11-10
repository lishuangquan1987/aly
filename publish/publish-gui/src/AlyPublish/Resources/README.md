# 本地化资源约定

发布 GUI 使用 [Irihi.Lingua](https://github.com/irihitech/Irihi.Lingua) 与 JSON 资源。默认语言文件是
`Strings.json`（英文，invariant），中文是 `Strings.zh-Hans.json`。

## 为什么不是 S001、S002

数字键无法维护：看到 `S047` 没人知道它属于哪个窗口、哪个控件。现在的键按 **页面 → 区域 → 字段** 分层，
JSON 保持嵌套，Lingua 编译后把层级用 `_` 连接成 C# 属性：

```json
{
  "ProjectTab": {
    "Publish": {
      "VersionLabel": "版本号",
      "ReleaseNotesPlaceholder": "本次发布的变更说明（必填）"
    }
  }
}
```

对应：

```xml
<TextBlock Text="{Translate {x:Static loc:LanguageManager+Keys.ProjectTab_Publish_VersionLabel}}" />
```

```csharp
LanguageManager.T(LanguageManager.Instance.ProjectTab_Publish_VersionLabel)
```

看到 `ProjectTab_Publish_VersionLabel` 就知道：项目页 → 发布面板 → 版本号标签。

## 命名规则

| 层级 | 含义 | 示例 |
|---|---|---|
| 第一层 | 页面 / 对话框 / 全局域 | `MainWindow`、`ProjectTab`、`AddProjectDialog`、`EditProjectDialog`、`Common`、`Message`、`Operation`、`Status` |
| 第二层 | 页面内的区域 | `Toolbar`、`ProjectList`、`Changes`、`Publish`、`History`、`Rules`、`Footer`、`Server`、`Local` |
| 第三层 | 字段 / 控件语义 | `SearchPlaceholder`、`RefreshTooltip`、`StageAll`、`VersionLabel`、`IgnoreFolders` |
| 第四层 | 仅在有歧义时使用 | `Upload.Table.Size`、`UnstagedEmpty.Title` |

约定：

- 窗口标题用 `WindowTitle`；页面大标题用 `Heading`；工具提示统一用 `Tooltip` 后缀。
- 输入框占位符统一 `Placeholder`；按钮文字统一 `Button` 或动词短语（`StageAll`、`Clear`）。
- 表格列头统一 `Table.Xxx`（如 `Changes.Table.File`、`History.Table.Version`）。
- 同一个页面里语义相同的控件可以复用同一个键；不同页面即使文案相同也分开，便于单独调整。
- 跨页面通用词放 `Common`（`Cancel`、`Back`、`Next`、`Refresh`、`Table.File`、`Language.Switch`），
  但不要滥用；能明确归属页面的不要放 `Common`。
- 错误/通知统一放 `Message` 下，按来源分组：`Message.Cli`、`Message.ProjectConfig`、`Message.Process`、
  `Message.Setup`、`Message.UpdateFolder`、`Message.Push`。
- 操作名称（用于进度/错误前缀）放 `Operation`，如 `Operation.Publish`、`Operation.StageAll`。

## 新增文案

1. 在 `Strings.json` 和 `Strings.zh-Hans.json` 的同一路径下添加键，两份文件的键集合必须完全一致。
2. XAML 用 `{Translate {x:Static loc:LanguageManager+Keys.Page_Area_Field}}`。
3. C# 用 `LanguageManager.T(LanguageManager.Instance.Page_Area_Field)`；带占位符用
   `LanguageManager.F(LanguageManager.Instance.Page_Area_Field, args)`。
4. `dotnet build` 会重新生成 `Keys` 与 `Instance` 属性；缺键会在编译期或运行时暴露为 `LINGUA` 错误/空串。
5. 改完中文后同步改英文；不要只改一份。

## 文件与语言

- 增加语言：复制 `Strings.zh-Hans.json` 为 `Strings.<culture>.json`，保持键集合一致，并在
  `Localization/LanguageManager.ResolveCulture` 中识别该 culture。
- `Strings.json` 必须保持英文/中性文案；它是未匹配 culture 的默认资源。
- 不要把中文写进 XAML 或 C# 字符串；日志（Serilog）可以保持中文，因为它不进入 UI。