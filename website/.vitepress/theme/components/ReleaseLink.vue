<script setup lang="ts">
import { computed } from 'vue'
import { findAsset, latestDownloadUrl } from '../data/releaseAssets'

const props = defineProps<{
  /** 稳定资产名（不带版本号），例如 AlyPublish-Setup.exe */
  asset: string
  /** 自定义显示文字 */
  label?: string
}>()

const url = computed(() => findAsset(props.asset)?.downloadUrl || latestDownloadUrl(props.asset))
</script>

<template>
  <a class="aly-inline-dl" :href="url" target="_blank" rel="noopener" :title="asset">
    <svg viewBox="0 0 16 16" aria-hidden="true">
      <path
        d="M8 1.5v8m0 0L5 6.5m3 3 3-3M2.5 11.5v2a1 1 0 0 0 1 1h9a1 1 0 0 0 1-1v-2"
        fill="none"
        stroke="currentColor"
        stroke-width="1.5"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </svg>
    <code>{{ label || asset }}</code>
  </a>
</template>

<style scoped>
.aly-inline-dl {
  display: inline-flex;
  align-items: center;
  gap: 0.15rem;
  color: var(--vp-c-brand-1);
  text-decoration: none;
  vertical-align: baseline;
}
.aly-inline-dl:hover {
  text-decoration: underline;
}
.aly-inline-dl code {
  background: transparent;
  padding: 0;
  color: inherit;
}
.aly-inline-dl svg {
  width: 0.85em;
  height: 0.85em;
  flex: none;
}
</style>
