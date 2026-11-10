<script setup lang="ts">
import { computed } from 'vue'
import { useData, withBase } from 'vitepress'

const { lang } = useData()
const isEn = computed(() => lang.value === 'en-US')
const prefix = computed(() => (isEn.value ? '/en' : ''))

const copy = computed(() =>
  isEn.value
    ? {
        kicker: 'Real interfaces',
        title: 'Screenshots from the running apps',
        lead: 'No concept art: the publish desktop app and the end-user update window, captured with demo data.',
        publishTag: 'Desktop publisher',
        publishTitle: 'Stage, publish and review history in one window',
        publishCaption: 'AlyPublish · demo project with staged and unstaged changes',
        clientTag: 'End-user window',
        clientTitle: 'Bottom-right update card, shown only when needed',
        clientCaption: 'aly-client-ui · update available, release notes included',
        note: 'Both windows support English and Chinese; the publish GUI can be switched from its toolbar.',
        links: { publish: '/docs/publish/gui', client: '/docs/client/ui' }
      }
    : {
        kicker: '真实界面',
        title: '来自正在运行的程序',
        lead: '不是概念图：发布桌面端与终端用户更新窗口，均用演示项目实际截取。',
        publishTag: '桌面发布器',
        publishTitle: '暂存、发布、版本历史，一屏完成',
        publishCaption: 'AlyPublish · 演示项目，含未暂存/已暂存变更',
        clientTag: '终端更新窗口',
        clientTitle: '右下角置顶更新卡片，按需出现',
        clientCaption: 'aly-client-ui · 发现新版本，含变更说明',
        note: '发布 GUI 与更新窗口都支持中文与英文；发布 GUI 可在工具栏切换语言。',
        links: { publish: '/docs/publish/gui', client: '/docs/client/ui' }
      }
)
</script>

<template>
  <section class="aly-showcase">
    <div class="aly-showcase-inner">
      <div class="aly-section-head">
        <span class="aly-kicker">{{ copy.kicker }}</span>
        <h2>{{ copy.title }}</h2>
        <p>{{ copy.lead }}</p>
      </div>

      <div class="aly-showcase-grid">
        <figure class="aly-shot-card">
          <a :href="withBase(prefix + copy.links.publish)">
            <img
              :src="withBase(isEn ? '/images/publish-gui-overview.webp' : '/images/publish-gui-overview-zh.webp')"
              :alt="copy.publishCaption"
              loading="lazy"
              decoding="async"
            />
          </a>
          <figcaption>
            <span class="aly-shot-tag">{{ copy.publishTag }}</span>
            <strong>{{ copy.publishTitle }}</strong>
            <span>{{ copy.publishCaption }}</span>
          </figcaption>
        </figure>

        <figure class="aly-shot-card is-client">
          <a :href="withBase(prefix + copy.links.client)">
            <img
              :src="withBase(isEn ? '/images/client-ui-update-en.webp' : '/images/client-ui-update-zh.webp')"
              :alt="copy.clientCaption"
              loading="lazy"
              decoding="async"
            />
          </a>
          <figcaption>
            <span class="aly-shot-tag">{{ copy.clientTag }}</span>
            <strong>{{ copy.clientTitle }}</strong>
            <span>{{ copy.clientCaption }}</span>
          </figcaption>
        </figure>
      </div>

      <p class="aly-showcase-note">{{ copy.note }}</p>
    </div>
  </section>
</template>