# Publish GUI Tool 需求确认

> 基于 publish-cli 的 GUI 包装工具，类似 Git GUI

---

## 已确认信息

- **后台调用方式**: 通过 Process.Start 调用 publish-cli.exe，解析 --json 输出
- **技术栈**: .NET 8 + Avalonia 12 + CommunityToolkit.Mvvm + Semi.Avalonia + Ursa.Avalonia
- **项目目录**: publish_tool_avalonia/（当前为空目录）

---

## 待确认问题

### Q1: 项目目录
publish_tool_avalonia/ 目录当前为空，是否直接在此目录下创建项目？还是需要新建一个不同的目录名？

回答：放到publish-gui目录下。把publish-cli和publish-gui都放到publish目录下。

### Q2: 功能范围
publish-cli 支持以下命令，GUI 需要覆盖哪些？

| 命令 | 说明 | 是否需要 |
|------|------|---------|
| status | 查看本地与服务端文件差异 | 需要 |
| dd / add --all | 添加文件到暂存区 | 需要 |
| 
eset / reset --all | 从暂存区移除文件 | 需要 |
| staged | 查看暂存区内容 | 不需要查看内容，因为都是二进制文件 |
| push | 推送暂存区文件到服务端 | 需要 |
| push-all | 一键推送所有变更 | 分两步：先添加暂存，再推送 |
| publish | 完整发布流程（status → add → push） | 是的 |
| log | 查看版本变更日志 | 需要 |
| watch | 实时监控文件变更 | 暂时不需要吧，由gui后台调用status来刷新 |
| config | 配置管理（init/set/get/list） | 需要init，init时需要指定服务端地址，projectname。gui可以直接添加已经init的项目，也可以添加未init的项目，配置服务端地址，projectname，然后后台调用init,写入配置 |
| project | 项目管理（create/update/delete/list/info） | 是的，可以在服务端创建项目 |
| server info | 查看服务端系统信息 | 是的 |

建议：核心功能 status + add + reset + staged + push + publish + log + config，project 管理和 watch 可以后续迭代。

回答：暂时不需要watch。

publish-cli发布流程：status->add->publish(填写版本号和commit message)

### Q3: UI 布局风格
类似哪种 Git GUI 工具的布局？

- **方案A**: 类 GitHub Desktop — 左侧文件列表（分 staged/unstaged），右侧提交区域（版本号+说明），下方历史日志
- **方案B**: 类 SourceTree — 上方文件列表，下方差异详情，右侧暂存区
- **方案C**: 类 GitKraken — 三栏布局（左：项目+分支信息，中：文件状态，右：提交详情）

回答：GitKraken 类型。publish-gui不用查看文件差异与比对文件差异，因为文件都是二进制，但是要显示文件在本地和服务端有多大

### Q4: publish-cli.exe 定位方式
GUI 如何找到 publish-cli.exe？

- **方案A**: 在设置中配置 publish-cli.exe 路径
- **方案B**: 默认查找同目录下的 publish-cli.exe，支持手动配置
- **方案C**: 从环境变量 PATH 中查找

回答：先从方案B中找，找不到再从方案C中找

### Q5: 配置管理交互
publish-cli 使用 .publish-cli/config.json 存储配置（server.url、project.name、project.path 等）。GUI 中：

- 是否需要一个"项目设置"页面来编辑这些配置？
- 还是通过 publish-cli config 命令间接管理？
- 首次使用是否需要一个"初始化向导"（引导用户配置 server URL、项目路径等）？

回答：1.需要项目设置来编辑这些配置。2.当修改配置保存时，调用publish-cli config命令保存。3.需要，分为两种向导，一种是已经init号的项目，一种是没有init好的项目

publish-cli.exe init之后，会在项目目录下生成一个.publish目录，里面有一个config.json，存放项目的配置。publish-gui调用publish-cli的时候，publish-cli会读取这些配置，优先使用命令行参数

### Q6: 与现有 publish_tool_avalonia 计划的关系
.trae/documents/avalonia-publish-tool-plan.md 中有一份详细的计划（直接调用 server API，不经过 publish-cli）。本次新建的 GUI 工具是：

- **方案A**: 完全替代该计划（新方案只通过 publish-cli 交互）
- **方案B**: 作为该计划的补充（publish-cli GUI 专注于发布工作流，原有计划的工具专注于项目/文件管理）
- **方案C**: 合并两者（GUI 既可以通过 publish-cli 调用，也可以直接调用 server API）

回答：我已经删除了.trae/documents/avalonia-publish-tool-plan.md，不用理它，从0开始

---

请逐一确认后我继续执行。
