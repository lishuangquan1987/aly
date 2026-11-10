# 多语言 SDK

Aly 的协议是语言无关的：`aly-client` 只输出统一的 JSON，SDK 负责参数拼接、超时、
进程收尸与更新编排。所有模型都从 `spec/aly-protocol.schema.json` 生成，并用
`spec/fixtures/scenarios` + 假客户端做一致性测试。

## 发布资产

Release 工作流会为每个 SDK 构建一个版本化 zip（`<version>` 为 Release 版本号）：

| 语言 | 资产 | 内容 |
|---|---|---|
| C# | `AlyClient.CSharpSDK-<version>.zip`（另有 `.nupkg`） | net40 + netstandard2.0 包 |
| TypeScript | `AlyClient.TypeScriptSDK-<version>.zip` | `dist` + 类型声明 + 源码 |
| Go | `AlyClient.GoSDK-<version>.zip` | Go module 源码 |
| Python | `AlyClient.PythonSDK-<version>.zip` | wheel + 源码 |
| Java | `AlyClient.JavaSDK-<version>.zip` | jar + 源码 |
| Rust | `AlyClient.RustSDK-<version>.zip` | `.crate` + 源码 |
| C++ | `AlyClient.CppSDK-<version>.zip` | 头文件 + 源码（Windows XP 方向） |
| Pascal | `AlyClient.PascalSDK-<version>.zip` | Free Pascal / Delphi mode 源码 |
| VB.NET | `AlyClient.VbNetSample-<version>.zip` | 示例工程 + 编译产物 |
| F# | `AlyClient.FSharpSample-<version>.zip` | 示例工程 + 编译产物 |
| COM | `AlyClient.Com-<version>.zip` | 32 位 DLL + 注册脚本（VB6/易语言） |

> 目前资产只发布到 GitHub Releases。npm / PyPI / Maven Central / crates.io / NuGet
> 的上架需在各平台配置凭据后再接入；zip 内已包含对应包格式（wheel、jar、`.crate`、
> `.nupkg`），便于后续直接推送。

## 能力对照

| 语言 | 底层 API | 托管生命周期 | 目标平台 |
|---|---|---|---|
| C# | `AlyApi` | `AlyUpdateClient` / `AlyUpdateUIClient` | net40（XP）、netstandard2.0 |
| TypeScript | Promise/回调 | `UpdateClient` | Node ≥18 / Electron |
| Go | context/回调 | `UpdateClient` | Go ≥1.21 |
| Python | 同步 API | `UpdateClient` | Python ≥3.9 |
| Java | 同步 API | `UpdateClient` | Java 8+ |
| Rust | `Api` + trait 回调 | `UpdateClient` | Rust stable |
| C++ | `Api` + 回调 | `UpdateClient` | Windows XP+ / C++11 |
| Pascal | `TApi` + 方法回调 | `TUpdateClient` | FPC 3.2.2，Windows XP+ |
| VB.NET / F# | C# SDK | C# SDK | .NET Framework 4.0+ |
| VB6 / 易语言 | COM | COM | 32 位 Windows + .NET 4.0 |

## 源码与文档

- [C# SDK](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.CSharpSDK)
- [TypeScript SDK](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.TypeScriptSDK)
- [Go SDK](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.GoSDK)
- [Python SDK](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.PythonSDK)
- [Java SDK](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.JavaSDK)
- [Rust SDK](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.RustSDK)
- [C++ SDK](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.CppSDK)
- [Pascal SDK](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.PascalSDK)
- [COM（VB6 / 易语言）](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.Com)

## 验证

仓库根目录提供统一打包脚本：

```powershell
tools/package-sdks/package-sdks.ps1 -Version 1.2.3 -Out artifacts/sdks
```

它会编译/打包每个 SDK，产物名与上表一致。各 SDK 的测试命令见对应 README。