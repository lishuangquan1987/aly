<script setup lang="ts">
import { computed } from 'vue'
import { useData } from 'vitepress'
import {
  assetsOf,
  describeAsset,
  formatDate,
  formatSize,
  release,
  type AssetGroup
} from '../data/releaseAssets'

const props = withDefaults(
  defineProps<{
    /** 只展示某一组资产；all = 全部三组 */
    group?: AssetGroup | 'all'
    /** 顶部是否显示版本徽标与发布时间 */
    showVersion?: boolean
    /** 底部是否显示「始终取最新版」提示 */
    showLatestHint?: boolean
    /** 紧凑模式（隐藏说明列） */
    compact?: boolean
  }>(),
  { group: 'all', showVersion: true, showLatestHint: true, compact: false }
)

const { lang } = useData()
const isEn = computed(() => lang.value === 'en-US')

const copy = computed(() =>
  isEn.value
    ? {
        version: 'Latest release',
        published: 'Published',
        file: 'File',
        note: 'Platform / contents',
        size: 'Size',
        groups: {
          package: 'AlyPublish package (GUI + CLI + updater)',
          server: 'aly-server',
          sdk: 'Language SDKs'
        } as Record<string, string>,
        hint: 'These links are pinned to',
        hintTail: 'You can also use',
        hintTail2: 'to always fetch the newest file.',
        releasePage: 'View all releases',
        empty: 'No release assets yet — please check the Releases page.'
      }
    : {
        version: '最新版本',
        published: '发布于',
        file: '文件',
        note: '平台 / 说明',
        size: '大小',
        groups: {
          package: 'AlyPublish 安装包（GUI + CLI + 更新器）',
          server: '服务端 aly-server',
          sdk: '多语言 SDK'
        } as Record<string, string>,
        hint: '以上链接固定到',
        hintTail: '也可以用',
        hintTail2: '始终取最新版。',
        releasePage: '查看全部 Release',
        empty: '暂无 Release 资产，请前往 Releases 页面查看。'
      }
)

const sections = computed(() => {
  const ids: AssetGroup[] = props.group === 'all'
    ? ['package', 'server', 'sdk']
    : [props.group as AssetGroup]
  return ids
    .map((id) => ({ id, title: copy.value.groups[id] || id, assets: assetsOf(id) }))
    .filter((s) => s.assets.length > 0)
})

const published = computed(() => formatDate(release.publishedAt, isEn.value))
</script>

<template>
  <div class="aly-rel">
    <div v-if="showVersion" class="aly-rel-head">
      <a class="aly-rel-badge" :href="release.htmlUrl" target="_blank" rel="noopener">
        {{ release.tag }}
      </a>
      <span class="aly-rel-head-text">
        {{ copy.version }}
        <template v-if="published"> · {{ copy.published }} {{ published }}</template>
      </span>
      <a class="aly-rel-release" :href="release.htmlUrl" target="_blank" rel="noopener">
        {{ copy.releasePage }} ↗
      </a>
    </div>

    <div v-for="section in sections" :key="section.id" class="aly-rel-section">
      <h4 v-if="sections.length > 1" class="aly-rel-title">{{ section.title }}</h4>
      <div class="aly-rel-scroll">
        <table class="aly-rel-table">
          <thead>
            <tr>
              <th>{{ copy.file }}</th>
              <th v-if="!compact">{{ copy.note }}</th>
              <th>{{ copy.size }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="asset in section.assets" :key="asset.name">
              <td>
                <a
                  class="aly-rel-link"
                  :href="asset.downloadUrl"
                  target="_blank"
                  rel="noopener"
                  :title="asset.name"
                >
                  <svg class="aly-rel-icon" viewBox="0 0 16 16" aria-hidden="true">
                    <path
                      d="M8 1.5v8m0 0L5 6.5m3 3 3-3M2.5 11.5v2a1 1 0 0 0 1 1h9a1 1 0 0 0 1-1v-2"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="1.4"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    />
                  </svg>
                  <code>{{ asset.name }}</code>
                </a>
              </td>
              <td v-if="!compact" class="aly-rel-note">
                {{ describeAsset(asset.name, isEn) }}
              </td>
              <td class="aly-rel-size">{{ formatSize(asset.size) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <p v-if="!sections.length" class="aly-rel-empty">{{ copy.empty }}</p>

    <p v-if="showLatestHint && sections.length" class="aly-rel-hint">
      {{ copy.hint }} <strong>{{ release.tag }}</strong>；{{ copy.hintTail }}
      <code>https://github.com/lishuangquan1987/aly/releases/latest/download/&lt;文件名&gt;</code>
      {{ copy.hintTail2 }}
    </p>
  </div>
</template>

<style scoped>
.aly-rel {
  margin: 1.5rem 0;
  border: 1px solid var(--vp-c-divider);
  border-radius: 12px;
  background: var(--vp-c-bg-soft);
  padding: 1rem 1.25rem;
}

.aly-rel-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.6rem;
  margin-bottom: 0.9rem;
  font-size: 0.9rem;
}

.aly-rel-badge {
  display: inline-block;
  padding: 0.15rem 0.6rem;
  border-radius: 999px;
  background: var(--vp-c-brand-1);
  color: #fff;
  font-weight: 600;
  font-size: 0.82rem;
  text-decoration: none;
}
.aly-rel-badge:hover {
  opacity: 0.9;
}

.aly-rel-head-text {
  color: var(--vp-c-text-2);
}

.aly-rel-release {
  margin-left: auto;
  font-size: 0.85rem;
  color: var(--vp-c-brand-1);
  text-decoration: none;
}
.aly-rel-release:hover {
  text-decoration: underline;
}

.aly-rel-section + .aly-rel-section {
  margin-top: 1.25rem;
}

.aly-rel-title {
  margin: 0 0 0.5rem;
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--vp-c-text-1);
}

.aly-rel-scroll {
  overflow-x: auto;
}

.aly-rel-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.88rem;
  display: table;
  margin: 0;
}

.aly-rel-table th,
.aly-rel-table td {
  border-top: 1px solid var(--vp-c-divider);
  border-bottom: 1px solid var(--vp-c-divider);
  padding: 0.5rem 0.6rem;
  text-align: left;
  vertical-align: middle;
}

.aly-rel-table th {
  color: var(--vp-c-text-2);
  font-weight: 600;
  white-space: nowrap;
}

.aly-rel-link {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  color: var(--vp-c-brand-1);
  text-decoration: none;
  font-weight: 500;
  white-space: nowrap;
}
.aly-rel-link:hover {
  text-decoration: underline;
}
.aly-rel-link code {
  background: transparent;
  padding: 0;
  color: inherit;
  font-size: 0.86rem;
}

.aly-rel-icon {
  width: 15px;
  height: 15px;
  flex: none;
}

.aly-rel-note {
  color: var(--vp-c-text-2);
  min-width: 12rem;
}

.aly-rel-size {
  white-space: nowrap;
  color: var(--vp-c-text-2);
  font-variant-numeric: tabular-nums;
}

.aly-rel-hint,
.aly-rel-empty {
  margin: 0.9rem 0 0;
  font-size: 0.82rem;
  color: var(--vp-c-text-2);
  line-height: 1.7;
}
.aly-rel-hint code {
  font-size: 0.8rem;
  word-break: break-all;
}
</style>
