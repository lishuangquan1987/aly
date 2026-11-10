---
title: 文档版本
description: 当前文档对应哪个代码版本，以及如何查看历史版本的文档。
---

# 文档版本

本站文档对应仓库 `master` 与最新 Release。所有页面描述的是当前实现，不保证与旧版本二进制逐字一致。

## 查看历史版本文档

文档与代码在同一个仓库、同一个 tag 里。需要某个旧版本时，直接在 GitHub 上切到对应 tag 阅读 `website/docs/`：

1. 打开 [Releases](https://github.com/lishuangquan1987/aly/releases) 找到部署的版本 tag；
2. 打开 `https://github.com/lishuangquan1987/aly/tree/<tag>/website/docs`；
3. 如需在本地渲染，检出该 tag 后执行 `cd website && pnpm install && pnpm dev`。

CLI 的参数与返回字段以对应版本的 `COMMANDS.md` 和源码为准；新版本文档可能已经删除旧参数。

## 当前版本

- 最新发布：[GitHub Releases](https://github.com/lishuangquan1987/aly/releases/latest)
- 文档源：[website/docs](https://github.com/lishuangquan1987/aly/tree/master/website/docs)
- 英文文档：[website/en/docs](https://github.com/lishuangquan1987/aly/tree/master/website/en/docs)

::: tip 升级前先对齐版本
服务端、发布工具与客户端可以独立升级，但客户端只接受**严格更高**的服务端版本。升级前请先阅读对应 Release 的变更说明。
:::