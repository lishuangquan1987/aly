# SDK release packaging

`package-sdks.ps1` builds every language SDK and writes one versioned zip per language
into `artifacts/sdks` (or `-Out`). It is used by `.github/workflows/release.yml`; the
resulting zips are attached to the GitHub Release.

## Usage

```powershell
tools/package-sdks/package-sdks.ps1 -Version 1.2.3 -Out artifacts/sdks
```

`-SkipCSharp` skips the C# zip because the release workflow's C# job already packs the
`.nupkg` and zips it.

## Output

| Zip | Artifact inside |
|---|---|
| `AlyClient.CSharpSDK-<version>.zip` | `.nupkg` + README |
| `AlyClient.TypeScriptSDK-<version>.zip` | `dist/`, `src/`, `test/`, `package.json` (version stamped) |
| `AlyClient.GoSDK-<version>.zip` | Go module source |
| `AlyClient.PythonSDK-<version>.zip` | wheel + source (version stamped) |
| `AlyClient.JavaSDK-<version>.zip` | jar + source |
| `AlyClient.RustSDK-<version>.zip` | `.crate` + source (version stamped) |
| `AlyClient.CppSDK-<version>.zip` | headers + source |
| `AlyClient.PascalSDK-<version>.zip` | source + build scripts |
| `AlyClient.VbNetSample-<version>.zip` | project + compiled output |
| `AlyClient.FSharpSample-<version>.zip` | project + compiled output |
| `AlyClient.Com-<version>.zip` | x86 DLL + registration scripts |

C#, TypeScript, Python, Java, Rust, VB.NET and F# packages carry the release version in
their native manifest where applicable. The Go/C++/Pascal/COM zips contain source and/or
a compiled COM DLL.

## Prerequisites

- .NET 8 SDK
- Node.js 20+
- Python 3.9+
- JDK 17
- Rust stable
- Go (only for the protocol check in CI)
- MinGW-w64 g++ (C++)
- Free Pascal 3.2.2 (Pascal)

All are installed by the release workflow's `build-sdk-packages` job.

## Publishing later

The zips already contain native package formats:

- C#: `.nupkg` (NuGet)
- TypeScript: `package.json` + `dist` (npm)
- Python: `.whl` (PyPI)
- Java: `.jar` (Maven Central)
- Rust: `.crate` (crates.io)

Publishing requires registry credentials and is intentionally not part of the release
workflow yet.