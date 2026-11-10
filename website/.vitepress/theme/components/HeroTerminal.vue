<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useData } from 'vitepress'

const { lang } = useData()
const isEn = computed(() => lang.value === 'en-US')
const ready = ref(false)
const root = ref<HTMLElement | null>(null)

const message = computed(() => (isEn.value ? 'Fix login timeout' : '修复登录超时'))

const lines = computed(() => [
  { kind: 'cmd', prompt: true, text: 'aly-publish status' },
  { kind: 'out', text: '  modified  src/App.exe' },
  { kind: 'out', text: '  new       assets/theme.pak' },
  { kind: 'cmd', prompt: true, text: 'aly-publish add --all' },
  { kind: 'cmd', prompt: true, text: `aly-publish push --version V1.0.2 --message "${message.value}"` },
  { kind: 'success', text: isEn.value ? '✓ V1.0.2 published · 3 files' : '✓ V1.0.2 已发布 · 3 个文件' },
  { kind: 'cmd', prompt: true, text: 'aly-client check_update' },
  { kind: 'json', text: '{"has_update":true,"new_version":"1.0.2"}' }
])

onMounted(() => {
  ready.value = true
})
</script>

<template>
  <div
    ref="root"
    class="aly-terminal"
    :class="{ 'is-ready': ready }"
    :aria-label="isEn ? 'Terminal demo of publishing and checking an update' : '发布与检查更新的终端演示'"
  >
    <div class="aly-terminal-bar">
      <span class="aly-terminal-dots" aria-hidden="true"><i /><i /><i /></span>
      <span class="aly-terminal-title">terminal — aly</span>
      <span class="aly-terminal-badge">JSON</span>
    </div>
    <pre class="aly-terminal-body"><span
      v-for="(line, index) in lines"
      :key="index"
      class="aly-terminal-line"
      :class="`is-${line.kind}`"
      :style="{ '--i': index }"
    ><span v-if="line.prompt" class="aly-terminal-prompt">$ </span>{{ line.text }}
</span></pre>
    <div class="aly-terminal-foot">
      <span>Windows XP</span>
      <span>Linux amd64 / arm64</span>
      <span>SQLite</span>
      <span>MIT</span>
    </div>
  </div>
</template>