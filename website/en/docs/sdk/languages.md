# Language SDKs

Aly's protocol is language-neutral: `aly-client` only emits a uniform JSON envelope,
while each SDK handles argument building, timeouts, process reaping and update
orchestration. All models are generated from `spec/aly-protocol.schema.json` and every
SDK is tested against `spec/fixtures/scenarios` with a fake client.

## Release assets

The release workflow builds a versioned zip per SDK. The table below is generated from the latest
release — click a file name to download; it refreshes on every publish:

<ReleaseDownloads group="sdk" />

> Assets currently go to GitHub Releases only. Publishing to npm / PyPI / Maven Central /
> crates.io / NuGet requires platform credentials; the zips already contain the native
> package formats (wheel, jar, `.crate`, `.nupkg`) so they can be pushed later.

## Capability matrix

| Language | Low-level API | Managed lifecycle | Target |
|---|---|---|---|
| C# | `AlyApi` | `AlyUpdateClient` / `AlyUpdateUIClient` | net40 (XP), netstandard2.0 |
| TypeScript | Promise/callback | `UpdateClient` | Node ≥18 / Electron |
| Go | context/callback | `UpdateClient` | Go ≥1.21 |
| Python | synchronous API | `UpdateClient` | Python ≥3.9 |
| Java | synchronous API | `UpdateClient` | Java 8+ |
| Rust | `Api` + trait callbacks | `UpdateClient` | Rust stable |
| C++ | `Api` + callbacks | `UpdateClient` | Windows XP+ / C++11 |
| Pascal | `TApi` + method callbacks | `TUpdateClient` | FPC 3.2.2, Windows XP+ |
| VB.NET / F# | C# SDK | C# SDK | .NET Framework 4.0+ |
| VB6 / EPL | COM | COM | 32-bit Windows + .NET 4.0 |

## Source and docs

- [C# SDK](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.CSharpSDK)
- [TypeScript SDK](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.TypeScriptSDK)
- [Go SDK](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.GoSDK)
- [Python SDK](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.PythonSDK)
- [Java SDK](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.JavaSDK)
- [Rust SDK](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.RustSDK)
- [C++ SDK](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.CppSDK)
- [Pascal SDK](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.PascalSDK)
- [COM (VB6 / EPL)](https://github.com/lishuangquan1987/aly/tree/master/client/aly-client-sdk/AlyClient.Com)

## Verification

The repository provides a unified packaging script:

```powershell
tools/package-sdks/package-sdks.ps1 -Version 1.2.3 -Out artifacts/sdks
```

It compiles/packages every SDK using the asset names above. Test commands live in each
SDK's README.