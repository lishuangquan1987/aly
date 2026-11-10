<script setup lang="ts">
import { computed } from 'vue'
import { useData, withBase } from 'vitepress'

const { lang } = useData()
const isEn = computed(() => lang.value === 'en-US')
const prefix = computed(() => (isEn.value ? '/en' : ''))

const copy = computed(() =>
  isEn.value
    ? {
        title: 'Ship the next release with three commands.',
        text: 'Start with the quick start, then open the module reference when you need a flag or a return field.',
        primary: 'Quick start',
        secondary: 'Command reference',
        github: 'Star on GitHub'
      }
    : {
        title: '把下一次发版，交给三条命令。',
        text: '先走一遍快速开始；需要查参数或返回字段时，直接打开模块参考。',
        primary: '快速开始',
        secondary: '命令参考',
        github: '在 GitHub 上 Star'
      }
)
</script>

<template>
  <section class="aly-cta">
    <div class="aly-cta-panel">
      <div class="aly-cta-glow" aria-hidden="true" />
      <h2>{{ copy.title }}</h2>
      <p>{{ copy.text }}</p>
      <div class="aly-cta-actions">
        <a class="aly-btn aly-btn-primary" :href="withBase(prefix + '/docs/getting-started')">{{ copy.primary }}</a>
        <a class="aly-btn" :href="withBase(prefix + '/docs/commands')">{{ copy.secondary }}</a>
        <a class="aly-btn aly-btn-ghost" href="https://github.com/lishuangquan1987/aly" target="_blank" rel="noopener">{{ copy.github }}</a>
      </div>
    </div>
  </section>
</template>