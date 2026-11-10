<script setup lang="ts">
import { computed } from 'vue'
import { useData, withBase } from 'vitepress'

const { lang, page } = useData()
const isEn = computed(() => lang.value === 'en-US')

const groups = {
  client: { zh: '更新器 aly-client', en: 'Updater aly-client', link: '/docs/client/' },
  publish: { zh: '发布工具 aly-publish', en: 'Publisher aly-publish', link: '/docs/publish' },
  server: { zh: '服务端 aly-server', en: 'Server aly-server', link: '/docs/server/deploy' },
  sdk: { zh: 'C# SDK', en: 'C# SDK', link: '/docs/sdk' }
} as const

const prefix = computed(() => (isEn.value ? '/en' : ''))

const crumbs = computed(() => {
  const relative = page.value.relativePath.replace(/^en\//, '')
  if (!relative.startsWith('docs/')) return []

  const segments = relative.slice('docs/'.length).split('/').filter(Boolean)
  if (segments.length === 0 || segments[0] === 'index.md') return []

  const root = {
    label: isEn.value ? 'Documentation' : '文档中心',
    link: `${prefix.value}/docs/`
  }

  const first = segments[0]
  const group = groups[first as keyof typeof groups]
  if (group && segments.length > 1) {
    const groupLabel = isEn.value ? group.en : group.zh
    const groupCrumb = { label: groupLabel, link: `${prefix.value}${group.link}` }
    const isGroupIndex = segments.length === 2 && segments[1] === 'index.md'
    if (isGroupIndex) return [root, { label: groupLabel, link: '' }]
    return [root, groupCrumb, { label: page.value.title || segments[segments.length - 1], link: '' }]
  }

  return [root, { label: page.value.title || segments[segments.length - 1], link: '' }]
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