<script setup lang="ts">
import { computed } from 'vue'
import { useData, withBase } from 'vitepress'

const { lang } = useData()
const isEn = computed(() => lang.value === 'en-US')
const prefix = computed(() => (isEn.value ? '/en' : ''))

const copy = computed(() =>
  isEn.value
    ? {
        kicker: 'Choose a path',
        title: 'Pick the role that fits you',
        lead: 'Three ways in, all with copyable commands and a short first run.',
        cards: [
          {
            icon: 'publish',
            title: 'Publish a release',
            text: 'Three commands: status, add, push. CLI or desktop GUI.',
            link: '/docs/publish',
            cta: 'Publish workflow'
          },
          {
            icon: 'integrate',
            title: 'Integrate the updater',
            text: 'C# SDK or any language that can spawn a process and read JSON.',
            link: '/docs/sdk',
            cta: 'Integration guide'
          },
          {
            icon: 'server',
            title: 'Deploy the server',
            text: 'One static binary and SQLite. Self-host it on your intranet.',
            link: '/docs/server/deploy',
            cta: 'Deployment guide'
          }
        ],
        stats: [
          { value: '9', label: 'updater commands' },
          { value: '19', label: 'publish commands' },
          { value: '16', label: 'REST endpoints' },
          { value: '135', label: 'E2E cases' }
        ]
      }
    : {
        kicker: '选一条路径',
        title: '先选角色，再看最短路径',
        lead: '三种入口都给出可复制命令和一次最小可用的跑通流程。',
        cards: [
          {
            icon: 'publish',
            title: '我要发布版本',
            text: 'status → add → push 三条命令；CLI 与桌面 GUI 共用配置。',
            link: '/docs/publish',
            cta: '发布工作流'
          },
          {
            icon: 'integrate',
            title: '我要接入更新器',
            text: 'C# SDK 直接编排，或任意语言拉起子进程解析 JSON。',
            link: '/docs/sdk',
            cta: '接入指南'
          },
          {
            icon: 'server',
            title: '我要部署服务端',
            text: '单文件 + SQLite，部署在内网，数据目录随程序走。',
            link: '/docs/server/deploy',
            cta: '部署指南'
          }
        ],
        stats: [
          { value: '9', label: '更新器命令' },
          { value: '19', label: '发布命令' },
          { value: '16', label: 'REST 接口' },
          { value: '135', label: '端到端用例' }
        ]
      }
)
</script>

<template>
  <section class="aly-roles">
    <div class="aly-roles-inner">
      <div class="aly-section-head">
        <span class="aly-kicker">{{ copy.kicker }}</span>
        <h2>{{ copy.title }}</h2>
        <p>{{ copy.lead }}</p>
      </div>

      <div class="aly-role-grid">
        <a
          v-for="card in copy.cards"
          :key="card.title"
          class="aly-role"
          :href="withBase(prefix + card.link)"
        >
          <span class="aly-role-icon" aria-hidden="true">
            <svg v-if="card.icon === 'publish'" viewBox="0 0 24 24" fill="none">
              <path d="M12 16V4m0 0L7.5 8.5M12 4l4.5 4.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
              <path d="M5 14v4.5A1.5 1.5 0 0 0 6.5 20h11a1.5 1.5 0 0 0 1.5-1.5V14" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
            </svg>
            <svg v-else-if="card.icon === 'integrate'" viewBox="0 0 24 24" fill="none">
              <path d="M8.5 9.5 6 12l2.5 2.5M15.5 9.5 18 12l-2.5 2.5M13.5 7l-3 10" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
              <rect x="3.5" y="4" width="17" height="16" rx="3" stroke="currentColor" stroke-width="1.6" />
            </svg>
            <svg v-else viewBox="0 0 24 24" fill="none">
              <rect x="4" y="4" width="16" height="7" rx="2" stroke="currentColor" stroke-width="1.6" />
              <rect x="4" y="13" width="16" height="7" rx="2" stroke="currentColor" stroke-width="1.6" />
              <path d="M8 7.5h.01M8 16.5h.01M12 7.5h4M12 16.5h4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
            </svg>
          </span>
          <h3>{{ card.title }}</h3>
          <p>{{ card.text }}</p>
          <span class="aly-role-cta">{{ card.cta }} <span aria-hidden="true">→</span></span>
        </a>
      </div>

      <dl class="aly-stats">
        <div v-for="stat in copy.stats" :key="stat.label">
          <dt>{{ stat.value }}</dt>
          <dd>{{ stat.label }}</dd>
        </div>
      </dl>
    </div>
  </section>
</template>