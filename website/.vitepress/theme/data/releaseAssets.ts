/**
 * 网站下载链接的数据层。
 * 数据由 scripts/gen-release-assets.mjs 在每次构建前从 GitHub Releases API 生成，
 * 仓库内的 json 是兜底快照。页面与组件只从这里取数，不硬编码资产名/版本号。
 */
import raw from './release-assets.json'

export interface ReleaseAsset {
  name: string
  size: number
  downloadUrl: string
  downloadCount: number
  contentType: string
}

export interface ReleaseData {
  tag: string
  publishedAt: string
  htmlUrl: string
  generatedAt: string
  assets: ReleaseAsset[]
}

export const release: ReleaseData = raw as ReleaseData

export const REPO_URL = 'https://github.com/lishuangquan1987/aly'
export const RELEASES_URL = `${REPO_URL}/releases`
export const LATEST_RELEASE_URL = `${REPO_URL}/releases/latest`
export const LATEST_DOWNLOAD_BASE = `${REPO_URL}/releases/latest/download`

export type AssetGroup = 'package' | 'server' | 'sdk' | 'other'

export function groupOf(name: string): AssetGroup {
  if (name.startsWith('AlyPublish-')) return 'package'
  if (name.startsWith('aly-server-')) return 'server'
  if (name.startsWith('AlyClient.')) return 'sdk'
  return 'other'
}

/** 固定指向最新版（不随构建快照变化），可作为备用链接。 */
export function latestDownloadUrl(name: string): string {
  return `${LATEST_DOWNLOAD_BASE}/${name}`
}

export function findAsset(name: string): ReleaseAsset | undefined {
  return release.assets.find((a) => a.name === name)
}

/** 资产名 -> 平台/说明。顺序即表格展示顺序。 */
const ASSET_INFO: Array<{ re: RegExp; zh: string; en: string }> = [
  // AlyPublish 安装包
  { re: /^AlyPublish-Setup\.exe$/, zh: 'Windows x64 · 安装包（开始菜单入口，自包含 .NET 8）', en: 'Windows x64 · installer (start-menu entry, self-contained .NET 8)' },
  { re: /^AlyPublish-win-x64\.zip$/, zh: 'Windows x64 · 便携包（解压即用）', en: 'Windows x64 · portable (unzip and run)' },
  { re: /^AlyPublish-linux-x64\.zip$/, zh: 'Linux x64 · 安装包（解压后运行 install.sh）', en: 'Linux x64 · package (unzip, then run install.sh)' },
  { re: /^AlyPublish-linux-arm64\.zip$/, zh: 'Linux arm64 · 安装包（解压后运行 install.sh）', en: 'Linux arm64 · package (unzip, then run install.sh)' },
  // 服务端
  { re: /^aly-server-windows-amd64\.exe$/, zh: 'Windows x64', en: 'Windows x64' },
  { re: /^aly-server-linux-amd64$/, zh: 'Linux x64', en: 'Linux x64' },
  { re: /^aly-server-linux-arm64$/, zh: 'Linux arm64', en: 'Linux arm64' },
  { re: /^aly-server-darwin-amd64$/, zh: 'macOS x64（未签名）', en: 'macOS x64 (unsigned)' },
  { re: /^aly-server-darwin-arm64$/, zh: 'macOS arm64（未签名）', en: 'macOS arm64 (unsigned)' },
  // SDK
  { re: /^AlyClient\.CSharpSDK.*\.nupkg$/, zh: 'C# SDK · NuGet 包（未上架 nuget.org）', en: 'C# SDK · NuGet package (not on nuget.org)' },
  { re: /^AlyClient\.CSharpSDK-.*\.zip$/, zh: 'C# SDK · 压缩包（包 + README）', en: 'C# SDK · zip (package + README)' },
  { re: /^AlyClient\.TypeScriptSDK/, zh: 'TypeScript / Node · dist + 类型声明', en: 'TypeScript / Node · dist + type declarations' },
  { re: /^AlyClient\.GoSDK/, zh: 'Go · module 源码', en: 'Go · module sources' },
  { re: /^AlyClient\.PythonSDK/, zh: 'Python · wheel + 源码', en: 'Python · wheel + sources' },
  { re: /^AlyClient\.JavaSDK/, zh: 'Java · jar + 源码', en: 'Java · jar + sources' },
  { re: /^AlyClient\.RustSDK/, zh: 'Rust · .crate + 源码', en: 'Rust · .crate + sources' },
  { re: /^AlyClient\.CppSDK/, zh: 'C++ · 头文件 + 源码（XP 方向）', en: 'C++ · headers + sources (XP target)' },
  { re: /^AlyClient\.PascalSDK/, zh: 'Free Pascal / Delphi · 源码', en: 'Free Pascal / Delphi · sources' },
  { re: /^AlyClient\.VbNetSample/, zh: 'VB.NET · 示例工程', en: 'VB.NET · sample project' },
  { re: /^AlyClient\.FSharpSample/, zh: 'F# · 示例工程', en: 'F# · sample project' },
  { re: /^AlyClient\.Com-/, zh: 'COM · 32 位 DLL + 注册脚本（VB6 / 易语言）', en: 'COM · 32-bit DLL + registration scripts (VB6 / EPL)' }
]

const FALLBACK_ZH = 'GitHub Release 资产'
const FALLBACK_EN = 'GitHub Release asset'

export function describeAsset(name: string, isEn: boolean): string {
  const hit = ASSET_INFO.find((x) => x.re.test(name))
  return hit ? (isEn ? hit.en : hit.zh) : isEn ? FALLBACK_EN : FALLBACK_ZH
}

function orderOf(name: string): number {
  const i = ASSET_INFO.findIndex((x) => x.re.test(name))
  return i < 0 ? 999 : i
}

export function assetsOf(group: AssetGroup | 'all'): ReleaseAsset[] {
  return release.assets
    .filter((a) => group === 'all' || groupOf(a.name) === group)
    .sort((a, b) => orderOf(a.name) - orderOf(b.name) || a.name.localeCompare(b.name, 'en'))
}

export function formatSize(bytes: number): string {
  if (!bytes || bytes < 0) return '—'
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${bytes} B`
}

export function formatDate(iso: string, isEn: boolean): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleDateString(isEn ? 'en-US' : 'zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    timeZone: 'UTC'
  })
}
