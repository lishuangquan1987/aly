<script setup lang="ts">
import { computed } from 'vue'
import { useData } from 'vitepress'

const { lang } = useData()
const isEn = computed(() => lang.value === 'en-US')

const copy = computed(() =>
  isEn.value
    ? {
        kicker: 'One update, end to end',
        title: 'From a push to an atomic replacement',
        steps: [
          { n: '01', title: 'Publish', cmd: 'aly-publish push', text: 'Scan the release directory, upload only changed files, record a version.' },
          { n: '02', title: 'Serve', cmd: 'aly-server', text: 'Keep the file manifest and change logs in SQLite plus the data directory.' },
          { n: '03', title: 'Check & download', cmd: 'aly-client download_update', text: 'Compare hashes, verify MD5 + SHA256, materialize unchanged files.' },
          { n: '04', title: 'Apply', cmd: 'apply_update', text: 'Three directory renames. A crash at any point recovers on next start.' }
        ]
      }
    : {
        kicker: '一次完整的更新',
        title: '从一次 push 到原子替换',
        steps: [
          { n: '01', title: '发布', cmd: 'aly-publish push', text: '扫描产物目录，只上传变更文件，登记版本与变更日志。' },
          { n: '02', title: '存储', cmd: 'aly-server', text: '发布清单、版本日志与文件保存在 SQLite 和数据目录。' },
          { n: '03', title: '检查与下载', cmd: 'aly-client download_update', text: '哈希比对，MD5 + SHA256 双校验，未变化文件提前物化。' },
          { n: '04', title: '应用', cmd: 'apply_update', text: '三次目录改名完成替换；任意时刻崩溃，下次启动自动恢复。' }
        ]
      }
)
</script>

<template>
  <section class="aly-flow">
    <div class="aly-flow-inner">
      <div class="aly-section-head">
        <span class="aly-kicker">{{ copy.kicker }}</span>
        <h2>{{ copy.title }}</h2>
      </div>
      <ol class="aly-flow-list">
        <li v-for="step in copy.steps" :key="step.n">
          <span class="aly-flow-n">{{ step.n }}</span>
          <h3>{{ step.title }}</h3>
          <code>{{ step.cmd }}</code>
          <p>{{ step.text }}</p>
        </li>
      </ol>
    </div>
  </section>
</template>