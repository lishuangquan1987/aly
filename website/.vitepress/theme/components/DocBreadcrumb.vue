<script setup lang="ts">
import { computed } from 'vue'
import { useData, withBase } from 'vitepress'

const { lang, page } = useData()
const isEn = computed(() => lang.value === 'en-US')
const prefix = computed(() => (isEn.value ? '/en' : ''))

// 侧栏五组的落地页，与 config.mts 的 sidebar 保持同一口径。
const GROUPS = {
  start: { zh: '开始使用', en: 'Start here', link: '/docs/getting-started' },
  guide: { zh: '指南', en: 'Guides', link: '/docs/guide/package' },
  reference: { zh: '参考', en: 'Reference', link: '/docs/commands' },
  concepts: { zh: '原理', en: 'Concepts', link: '/docs/concepts' },
  help: { zh: '帮助', en: 'Help', link: '/docs/troubleshooting' }
} as const

type GroupKey = keyof typeof GROUPS

// 每个页面归到哪一组：目录名不足以表达归属（发布/接入/部署分散在多个目录），所以按页面显式映射。
const PAGE_GROUPS: Record<string, GroupKey> = {
  'getting-started': 'start',
  download: 'start',

  'guide/package': 'guide',
  publish: 'guide',
  'publish/gui': 'guide',
  'server/deploy': 'guide',
  'client/index': 'guide',
  'client/ui': 'guide',
  sdk: 'guide',

  commands: 'reference',
  'publish/commands-config': 'reference',
  'publish/commands-project': 'reference',
  'publish/commands-workflow': 'reference',
  'client/config': 'reference',
  api: 'reference',
  'server/api-project': 'reference',
  'server/api-file': 'reference',
  'server/api-models': 'reference',
  'sdk/api': 'reference',
  'sdk/languages': 'reference',

  concepts: 'concepts',
  architecture: 'concepts',
  'self-update': 'concepts',
  build: 'concepts',
  testing: 'concepts',

  troubleshooting: 'help',
  faq: 'help',
  versions: 'help'
}

function groupOf(path: string): GroupKey | null {
  if (path.startsWith('start/')) return 'start'
  return PAGE_GROUPS[path] ?? null
}

const crumbs = computed(() => {
  const relative = page.value.relativePath.replace(/^en\//, '')
  if (!relative.startsWith('docs/')) return []

  const path = relative.slice('docs/'.length).replace(/\.md$/, '')
  if (path === 'index' || path === '') return []

  const root = { label: isEn.value ? 'Documentation' : '文档中心', link: '/docs/' }
  const pageTitle = page.value.title || path.split('/').pop() || ''

  const key = groupOf(path)
  if (!key) return [root, { label: pageTitle, link: '' }]

  const group = GROUPS[key]
  const groupLabel = isEn.value ? group.en : group.zh
  const groupLink = `${prefix.value}${group.link}`

  // 组落地页自身不再指向自己。
  const landing = group.link.replace(/^\/docs\//, '')
  if (path === landing) return [root, { label: pageTitle, link: '' }]

  return [root, { label: groupLabel, link: groupLink }, { label: pageTitle, link: '' }]
})
</script>

<template>
  <nav v-if="crumbs.length" class="aly-breadcrumb" aria-label="Breadcrumb">
    <template v-for="(crumb, index) in crumbs" :key="`${crumb.label}-${index}`">
      <span v-if="index > 0" class="sep" aria-hidden="true">/</span>
      <a v-if="crumb.link && index < crumbs.length - 1" :href="withBase(crumb.link)">{{ crumb.label }}</a>
      <span v-else :aria-current="index === crumbs.length - 1 ? 'page' : undefined">{{ crumb.label }}</span>
    </template>
  </nav>
</template>
