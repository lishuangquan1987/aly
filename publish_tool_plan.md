# 客户端UI优化计划

## 概述
基于项目实际功能，对客户端UI进行优化，提升用户体验。

---

## 优化任务清单

### [x] 1. 文件列表优化（优先级高）
- [x] 1.1 显示文件大小（使用BytesToSizeConverter）
- [x] 1.2 差异状态标识（新增/修改/无变化）
- [x] 1.3 全选功能

### [x] 2. 上传队列优化（优先级高）
- [x] 2.1 文件状态图标显示
- [x] 2.2 进度统计显示
- [x] 2.3 失败重试入口

### [x] 3. 服务器信息栏优化（中优先级）
- [x] 3.1 磁盘显示格式改为"已用/总计 GB"
- [x] 3.2 悬停提示显示完整信息

### [x] 4. 工具栏分组优化（中优先级）
- [x] 4.1 按功能分组显示
- [x] 4.2 视觉分隔符区分

### [x] 5. 项目设置对话框增强（中优先级）
- [x] 5.1 显示服务器地址（只读）
- [x] 5.2 忽略配置快捷入口

### [x] 6. 底部操作栏优化（低优先级）
- [x] 6.1 添加停止按钮
- [x] 6.2 优化布局间距

---

## 执行记录

| 日期 | 任务 | 状态 |
|------|------|------|
| 2026-05-20 | 创建优化计划文档 | ✅ 完成 |
| 2026-05-20 | 1.1 LocalFileItem添加FileSize属性 | ✅ 完成 |
| 2026-05-20 | 1.2 文件列表添加文件大小列 | ✅ 完成 |
| 2026-05-20 | 1.3 添加差异状态标识 | ✅ 完成 |
| 2026-05-20 | 1.4 添加全选功能 | ✅ 完成 |
| 2026-05-20 | 2.1 上传队列文件状态图标 | ✅ 完成 |
| 2026-05-20 | 2.2 上传队列进度统计 | ✅ 完成 |
| 2026-05-20 | 2.3 失败重试入口 | ✅ 完成 |
| 2026-05-20 | 3.1 服务器信息栏磁盘格式优化 | ✅ 完成 |
| 2026-05-20 | 4.1 工具栏分组优化 | ✅ 完成 |
| 2026-05-20 | 5.1 项目设置对话框增强 | ✅ 完成 |
| 2026-05-20 | 6.1 底部操作栏停止按钮 | ✅ 完成 |

---

## 新增文件

- Converters/CompareStatusConverter.cs
- Converters/StatusToColorConverter.cs
- Converters/UploadStatusToTextConverter.cs
- Converters/UploadStatusToColorConverter.cs
- Converters/UploadStatusToFailedConverter.cs
- Converters/DiskInfoFormatterConverter.cs
- Converters/DiskInfoTooltipConverter.cs

## 修改文件

- Models/Local/LocalFileItem.cs - 添加FileSize、CompareStatus属性
- Services/LocalFileService.cs - 填充文件大小和状态
- ViewModels/ProjectPageViewModel.cs - 添加全选、进度统计、重试命令
- ViewModels/ProjectSettingsDialogViewModel.cs - 添加服务器地址和忽略配置入口
- Views/Controls/ProjectPage.axaml - 优化文件列表、工具栏、底部操作栏
- Views/Controls/UploadQueuePanel.axaml - 优化上传队列显示
- Views/Dialogs/ProjectSettingsDialog.axaml - 增强对话框
- Views/Dialogs/ProjectSettingsDialog.axaml.cs - 处理忽略配置事件
- App.axaml - 注册新Converter