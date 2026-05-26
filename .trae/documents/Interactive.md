# Pages 文件夹重构 - 待确认问题

## 当前状态分析

| 文件组 | View 类型 | View 命名 | 打开方式 | 代码后置 |
|--------|----------|-----------|---------|---------|
| **AddProjectDialog** ✅ | UserControl | `AddProjectDialogView` | `Dialog.ShowStandardAsync` (Ursa) | 极简 |
| AddServerProjectDialog | Window | `AddServerProjectDialog` (无View后缀) | `MainWindow.axaml.cs` 手动 new + ShowDialog | 有事件处理 |
| ChangeLogsDialog | Window | `ChangeLogsDialog` | `MainWindow.axaml.cs` 手动 new + ShowDialog | 有Click事件 |
| ConfigEditorDialog | Window | `ConfigEditorDialog` | `MainWindow.axaml.cs` 手动 new + ShowDialog | 有Click事件 |
| DeleteConfirmDialog | Window | `DeleteConfirmDialog` | `MainWindow.axaml.cs` 手动 new + ShowDialog\<bool\> | 有Click事件 |
| ProjectSettingsDialog | Window | `ProjectSettingsDialog` | `MainWindow.axaml.cs` 手动 new + ShowDialog | 复杂事件处理 |

ViewLocator 的命名转换规则：
`ViewModels.XxxViewModel` → `Views.XxxView`

所以 `AddServerProjectDialogViewModel` 会自动查找 `AddServerProjectDialogView`（而非当前的 `AddServerProjectDialog`）

---

## 待确认问题

### Q1: View 文件命名
是否所有 View 统一加 `View` 后缀以适配 ViewLocator？
- `AddServerProjectDialog.axaml` → `AddServerProjectDialogView.axaml`
- `ChangeLogsDialog.axaml` → `ChangeLogsDialogView.axaml`
- `ConfigEditorDialog.axaml` → `ConfigEditorDialogView.axaml`
- `DeleteConfirmDialog.axaml` → `DeleteConfirmDialogView.axaml`
- `ProjectSettingsDialog.axaml` → `ProjectSettingsDialogView.axaml`

回复：是的，不然ViewLocator无法使用

### Q2: 对话框打开方式
AddProjectDialog 是用 **Ursa Dialog**（`Dialog.ShowStandardAsync`）打开的，当前其他对话框是在 `MainWindow.axaml.cs` 中手动 `new Window + ShowDialog` 打开的。

是否所有对话框都改为 UserControl + 由 ViewModel 层通过 Ursa Dialog 打开（参考 AddProjectDialog 的用法）？

如果是，那 `MainWindow.axaml.cs` 中的 `ShowXxxDialog` 方法和 `ProjectPage.axaml.cs` 中的事件连线代码都将被移除，改为在 ViewModel 中直接调用 `Dialog.ShowStandardAsync`。
回复：使用**Ursa Dialog**打开

### Q3: DeleteConfirmDialog 的返回值处理
当前 `DeleteConfirmDialog` 使用 `ShowDialog<bool>` 返回用户选择。如果改为 UserControl + Ursa Dialog 模式，建议在 ViewModel 中加一个 `IsConfirmed` 属性，`Confirm` 命令设置该属性后触发 `CloseRequested`，调用方读取该属性判断结果。这个方案可以吗？

回复：不可行。查看Ursa Dialog中的弹框方法，有很多，总有一种适合这个场景

### Q4: ProjectSettingsDialog 中的子对话框
`ProjectSettingsDialog` 内部会打开 `ConfigEditorDialog`（通过 ViewModel 的 `OpenIgnoreConfigRequested` 事件 → code-behind 手动创建）。重构后应该直接在 `ProjectSettingsDialogViewModel` 中通过 Ursa Dialog 打开 `ConfigEditorDialogView`，对吗？

回复：是的

---

请逐一确认后我继续执行。
