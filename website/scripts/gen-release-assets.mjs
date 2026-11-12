#!/usr/bin/env node
/**
 * 生成网站「下载」页使用的 Release 资产清单。
 *
 * 数据来源：GitHub Releases API（最新 release，或 ALY_RELEASE_TAG 指定的 tag）。
 * 输出：website/.vitepress/theme/data/release-assets.json
 *
 * 每次网站构建（push / 手动触发 / 发布 release 后重新部署）都会重新拉取，
 * 因此页面上的下载链接与版本号始终跟随最新 Release 更新。
 * 仓库里提交的 json 是兜底快照；API 临时不可用或资产尚未上传完时沿用该快照，
 * 不会让站点构建失败或渲染出空下载页。
 *
 * 环境变量：
 *   ALY_RELEASE_TAG  可选，指定 tag（例如 v0.1.11）；留空用 /releases/latest
 *   GITHUB_TOKEN     可选，提高 API 限额（CI 里传 github.token）
 */
import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const OWNER = 'lishuangquan1987'
const REPO = 'aly'
const OUT = join(
  dirname(fileURLToPath(import.meta.url)),
  '../.vitepress/theme/data/release-assets.json'
)

const tag = (process.env.ALY_RELEASE_TAG || '').trim()
const endpoint = tag
  ? `https://api.github.com/repos/${OWNER}/${REPO}/releases/tags/${encodeURIComponent(tag)}`
  : `https://api.github.com/repos/${OWNER}/${REPO}/releases/latest`

const headers = {
  Accept: 'application/vnd.github+json',
  'X-GitHub-Api-Version': '2022-11-28',
  'User-Agent': 'aly-website-build'
}
if (process.env.GITHUB_TOKEN) {
  headers.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`
}

function hasFallback() {
  if (!existsSync(OUT)) return false
  try {
    const old = JSON.parse(readFileSync(OUT, 'utf8'))
    return Array.isArray(old.assets) && old.assets.length > 0
  } catch {
    return false
  }
}

async function main() {
  let data
  try {
    const res = await fetch(endpoint, { headers })
    if (!res.ok) throw new Error(`HTTP ${res.status} ${res.statusText}`)
    const rel = await res.json()

    const assets = (rel.assets || [])
      .map((a) => ({
        name: a.name,
        size: a.size ?? 0,
        downloadUrl: a.browser_download_url,
        downloadCount: a.download_count ?? 0,
        contentType: a.content_type || ''
      }))
      .filter((a) => a.name && a.downloadUrl)
      .sort((a, b) => a.name.localeCompare(b.name, 'en'))

    // 发布瞬间资产可能还在上传：视为失败并沿用快照，等上传完成后再次部署会刷新。
    if (assets.length === 0) throw new Error('该 Release 还没有资产（可能正在上传）')

    data = {
      tag: rel.tag_name,
      publishedAt: rel.published_at || '',
      htmlUrl: rel.html_url || `https://github.com/${OWNER}/${REPO}/releases/tag/${rel.tag_name}`,
      generatedAt: new Date().toISOString(),
      assets
    }
  } catch (err) {
    if (hasFallback()) {
      console.warn(`::warning::拉取 Release 资产失败（${err.message}），沿用仓库内的资产快照。`)
      return
    }
    console.error(`拉取 Release 资产失败且没有可用快照：${err.message}`)
    process.exit(1)
  }

  writeFileSync(OUT, `${JSON.stringify(data, null, 2)}\n`)
  console.log(`已生成 ${data.assets.length} 个下载链接：${data.tag}（发布于 ${data.publishedAt}）`)
}

main()
