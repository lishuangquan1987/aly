<script setup lang="ts">
import { computed } from 'vue'
import { useData, withBase } from 'vitepress'

const { lang } = useData()
const isEn = computed(() => lang.value === 'en-US')
const prefix = computed(() => (isEn.value ? '/en' : ''))

const copy = computed(() =>
  isEn.value
    ? {
        kicker: 'Documentation',
        title: 'Start from your role, then go as deep as you need',
        lead: 'Every path below has a copyable first run. The module reference keeps parameters, return fields and failure behavior next to the code.',
        roles: [
          { title: 'Publish a release', text: 'CLI or desktop GUI, three commands.', link: '/docs/publish', cta: 'Publish workflow' },
          { title: 'Integrate the updater', text: 'C# SDK or any language over JSON.', link: '/docs/sdk', cta: 'Integration' },
          { title: 'Deploy the server', text: 'One binary, SQLite, intranet ready.', link: '/docs/server/deploy', cta: 'Deployment' },
          { title: 'Look up a command', text: 'Every flag and every return field.', link: '/docs/commands', cta: 'Command reference' }
        ],
        modules: [
          { name: 'aly-client', text: 'Updater on end-user machines', link: '/docs/client/' },
          { name: 'aly-publish', text: 'Publisher CLI and desktop GUI', link: '/docs/publish' },
          { name: 'aly-server', text: 'REST API, SQLite, data directory', link: '/docs/server/deploy' },
          { name: 'C# SDK', text: 'Managed orchestration for .NET hosts', link: '/docs/sdk' }
        ],
        start: 'First time here?',
        startText: 'The quick start picks your OS and host language, then shows only the shortest working path.',
        startCta: 'Open quick start'
      }
    : {
        kicker: '文档中心',
        title: '先选角色，再按需深入',
        lead: '每条路径都给出可直接复制的第一次跑通；模块参考把参数、返回字段和失败行为放在代码旁边。',
        roles: [
          { title: '发布新版本', text: 'CLI 或桌面 GUI，三条命令完成。', link: '/docs/publish', cta: '发布工作流' },
          { title: '接入更新器', text: 'C# SDK，或任意语言通过 JSON 接入。', link: '/docs/sdk', cta: '接入指南' },
          { title: '部署服务端', text: '单文件 + SQLite，内网自托管。', link: '/docs/server/deploy', cta: '部署指南' },
          { title: '查命令与接口', text: '每条命令的参数、返回字段与失败行为。', link: '/docs/commands', cta: '命令参考' }
        ],
        modules: [
          { name: 'aly-client', text: '终端机器上的更新器', link: '/docs/client/' },
          { name: 'aly-publish', text: '发布 CLI 与桌面 GUI', link: '/docs/publish' },
          { name: 'aly-server', text: 'REST API、SQLite 与数据目录', link: '/docs/server/deploy' },
          { name: 'C# SDK', text: '.NET 宿主的管理式编排', link: '/docs/sdk' }
        ],
        start: '第一次使用？',
        startText: '快速开始会先让你选操作系统与宿主语言，然后只展示最短可用路径。',
        startCta: '打开快速开始'
      }
)
</script>

<template>
  <section class="aly-doc-hub">
    <div class="aly-doc-intro">
      <span class="aly-kicker">{{ copy.kicker }}</span>
      <h1>{{ copy.title }}</h1>
      <p>{{ copy.lead }}</p>
    </div>

    <div class="aly-doc-grid">
      <a v-for="role in copy.roles" :key="role.title" class="aly-doc-card" :href="withBase(prefix + role.link)">
        <strong>{{ role.title }}</strong>
        <span>{{ role.text }}</span>
        <em>{{ role.cta }} <span aria-hidden="true">→</span></em>
      </a>
    </div>

    <div class="aly-doc-modules">
      <a v-for="mod in copy.modules" :key="mod.name" :href="withBase(prefix + mod.link)">
        <code>{{ mod.name }}</code>
        <span>{{ mod.text }}</span>
      </a>
    </div>

    <div class="aly-doc-start">
      <div>
        <strong>{{ copy.start }}</strong>
        <span>{{ copy.startText }}</span>
      </div>
      <a class="aly-btn aly-btn-primary" :href="withBase(prefix + '/docs/getting-started')">{{ copy.startCta }}</a>
    </div>
  </section>
</template>