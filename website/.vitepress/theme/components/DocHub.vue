<script setup lang="ts">
import { computed } from 'vue'
import { useData, withBase } from 'vitepress'

const { lang } = useData()
const isEn = computed(() => lang.value === 'en-US')
const prefix = computed(() => (isEn.value ? '/en' : ''))

const copy = computed(() =>
  isEn.value
    ? {
        kicker: 'Quick start',
        title: 'What do you want to do?',
        lead: 'Each path keeps only the shortest working commands. Principles and every flag live in the module reference.',
        roles: [
          { title: 'Publish a release', text: 'Install one package, click through the GUI, ship V1.0.1.', link: '/docs/start/publish', cta: 'Start publishing' },
          { title: 'Make the app self-update', text: 'Launch the updater from your host and verify one update locally.', link: '/docs/start/integrate', cta: 'Integrate the updater' },
          { title: 'Deploy the server', text: 'One binary plus SQLite, self-hosted on your intranet.', link: '/docs/start/server', cta: 'Deploy the server' }
        ],
        modules: [
          { name: 'aly-client', text: 'Updater on end-user machines', link: '/docs/client/' },
          { name: 'aly-publish', text: 'Publisher CLI and desktop GUI', link: '/docs/publish' },
          { name: 'aly-server', text: 'REST API, SQLite, data directory', link: '/docs/server/deploy' },
          { name: 'C# SDK', text: 'Managed orchestration for .NET hosts', link: '/docs/sdk' }
        ],
        start: 'Already running?',
        startText: 'Jump straight to the reference for flags, return fields and failure behavior.',
        startCta: 'Open the reference'
      }
    : {
        kicker: '快速开始',
        title: '你要做什么？',
        lead: '三条路径都只给最短可用命令；原理与全量参数在各模块的参考页里。',
        roles: [
          { title: '发布新版本', text: '装一个包，GUI 点几下把 V1.0.1 发出去。', link: '/docs/start/publish', cta: '开始发布' },
          { title: '让应用自动更新', text: '宿主拉起更新器，本机先跑通一次完整更新。', link: '/docs/start/integrate', cta: '接入更新器' },
          { title: '部署服务端', text: '一个二进制加 SQLite，内网自托管。', link: '/docs/start/server', cta: '部署服务端' }
        ],
        modules: [
          { name: 'aly-client', text: '终端机器上的更新器', link: '/docs/client/' },
          { name: 'aly-publish', text: '发布 CLI 与桌面 GUI', link: '/docs/publish' },
          { name: 'aly-server', text: 'REST API、SQLite 与数据目录', link: '/docs/server/deploy' },
          { name: 'C# SDK', text: '.NET 宿主的管理式编排', link: '/docs/sdk' }
        ],
        start: '已经跑通了？',
        startText: '直接去参考页查参数、返回字段与失败行为。',
        startCta: '打开参考'
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
      <a class="aly-btn aly-btn-primary" :href="withBase(prefix + '/docs/commands')">{{ copy.startCta }}</a>
    </div>
  </section>
</template>
