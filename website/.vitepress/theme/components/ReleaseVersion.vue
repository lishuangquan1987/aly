<script setup lang="ts">
import { computed } from 'vue'
import { useData } from 'vitepress'
import { formatDate, release, RELEASES_URL } from '../data/releaseAssets'

const { lang } = useData()
const isEn = computed(() => lang.value === 'en-US')
const published = computed(() => formatDate(release.publishedAt, isEn.value))
</script>

<template>
  <div class="aly-rel-version">
    <a class="aly-rel-version-tag" :href="release.htmlUrl" target="_blank" rel="noopener">
      {{ release.tag }}
    </a>
    <span>
      {{ isEn ? 'is the current release' : '是当前最新版本' }}
      <template v-if="published">
        · {{ isEn ? 'published' : '发布于' }} {{ published }}
      </template>
    </span>
    <a :href="RELEASES_URL" target="_blank" rel="noopener">
      {{ isEn ? 'All releases ↗' : '全部 Release ↗' }}
    </a>
  </div>
</template>

<style scoped>
.aly-rel-version {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.55rem;
  margin: 1rem 0;
  font-size: 0.9rem;
  color: var(--vp-c-text-2);
}
.aly-rel-version-tag {
  padding: 0.1rem 0.6rem;
  border-radius: 999px;
  background: var(--vp-c-brand-1);
  color: #fff;
  font-weight: 600;
  text-decoration: none;
}
.aly-rel-version a:last-child {
  margin-left: auto;
  color: var(--vp-c-brand-1);
  text-decoration: none;
}
.aly-rel-version a:last-child:hover {
  text-decoration: underline;
}
</style>
