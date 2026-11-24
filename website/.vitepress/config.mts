import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
const repoUrl = 'https://github.com/lishuangquan1987/aly'
const siteUrl = 'https://lishuangquan1987.github.io/aly/'

const zhSidebar = [
  {
    text: '开始使用',
    items: [
      { text: '文档中心', link: '/docs/' },
      { text: '快速开始', link: '/docs/getting-started' },
      { text: '发布第一个版本', link: '/docs/start/publish' },
      { text: '接入更新器', link: '/docs/start/integrate' },
      { text: '部署服务端', link: '/docs/start/server' },
      { text: '下载与安装', link: '/docs/download' }
    ]
  },
  {
    text: '指南',
    items: [
      { text: '发行包结构与分发', link: '/docs/guide/package' },
      { text: '打包与发布', link: '/docs/publish' },
      { text: '发布 GUI', link: '/docs/publish/gui' },
      { text: '部署与运维', link: '/docs/server/deploy' },
      { text: '宿主集成', link: '/docs/client/' },
      { text: '更新窗口', link: '/docs/client/ui' },
      { text: 'C# SDK 集成', link: '/docs/sdk' }
    ]
  },
  {
    text: '参考',
    items: [
      { text: 'aly-client 命令', link: '/docs/commands' },
      { text: '配置命令', link: '/docs/publish/commands-config' },
      { text: '项目命令', link: '/docs/publish/commands-project' },
      { text: '发布流程命令', link: '/docs/publish/commands-workflow' },
      { text: '配置文件参考', link: '/docs/client/config' },
      { text: '服务端 API 总览', link: '/docs/api' },
      { text: '项目接口', link: '/docs/server/api-project' },
      { text: '文件接口', link: '/docs/server/api-file' },
      { text: '数据模型', link: '/docs/server/api-models' },
      { text: 'C# SDK API', link: '/docs/sdk/api' },
      { text: '多语言 SDK', link: '/docs/sdk/languages' }
    ]
  },
  {
    text: '原理',
    items: [
      { text: '核心概念', link: '/docs/concepts' },
      { text: '系统架构', link: '/docs/architecture' },
      { text: '自更新设计', link: '/docs/self-update' },
      { text: '构建与跨平台', link: '/docs/build' },
      { text: '测试体系', link: '/docs/testing' }
    ]
  },
  {
    text: '帮助',
    items: [
      { text: '排障', link: '/docs/troubleshooting' },
      { text: 'FAQ', link: '/docs/faq' },
      { text: '文档版本', link: '/docs/versions' }
    ]
  }
]

const enSidebar = [
  {
    text: 'Start here',
    items: [
      { text: 'Documentation', link: '/en/docs/' },
      { text: 'Quick Start', link: '/en/docs/getting-started' },
      { text: 'Publish your first release', link: '/en/docs/start/publish' },
      { text: 'Integrate the updater', link: '/en/docs/start/integrate' },
      { text: 'Deploy the server', link: '/en/docs/start/server' },
      { text: 'Download & install', link: '/en/docs/download' }
    ]
  },
  {
    text: 'Guides',
    items: [
      { text: 'Package layout & distribution', link: '/en/docs/guide/package' },
      { text: 'Packaging & publishing', link: '/en/docs/publish' },
      { text: 'Publish GUI', link: '/en/docs/publish/gui' },
      { text: 'Deployment & operations', link: '/en/docs/server/deploy' },
      { text: 'Host integration', link: '/en/docs/client/' },
      { text: 'Update window', link: '/en/docs/client/ui' },
      { text: 'C# SDK integration', link: '/en/docs/sdk' }
    ]
  },
  {
    text: 'Reference',
    items: [
      { text: 'aly-client commands', link: '/en/docs/commands' },
      { text: 'Config commands', link: '/en/docs/publish/commands-config' },
      { text: 'Project commands', link: '/en/docs/publish/commands-project' },
      { text: 'Workflow commands', link: '/en/docs/publish/commands-workflow' },
      { text: 'Configuration files', link: '/en/docs/client/config' },
      { text: 'Server API', link: '/en/docs/api' },
      { text: 'Project API', link: '/en/docs/server/api-project' },
      { text: 'File API', link: '/en/docs/server/api-file' },
      { text: 'Data models', link: '/en/docs/server/api-models' },
      { text: 'C# SDK API', link: '/en/docs/sdk/api' },
      { text: 'Language SDKs', link: '/en/docs/sdk/languages' }
    ]
  },
  {
    text: 'Concepts',
    items: [
      { text: 'Core concepts', link: '/en/docs/concepts' },
      { text: 'Architecture', link: '/en/docs/architecture' },
      { text: 'Self-update design', link: '/en/docs/self-update' },
      { text: 'Build & cross-platform', link: '/en/docs/build' },
      { text: 'Testing', link: '/en/docs/testing' }
    ]
  },
  {
    text: 'Help',
    items: [
      { text: 'Troubleshooting', link: '/en/docs/troubleshooting' },
      { text: 'FAQ', link: '/en/docs/faq' },
      { text: 'Documentation versions', link: '/en/docs/versions' }
    ]
  }
]

const zhNav = [
  { text: '文档', link: '/docs/', activeMatch: '^/docs/$' },
  { text: '快速开始', link: '/docs/getting-started' },
  {
    text: '参考',
    items: [
      { text: 'aly-client 命令', link: '/docs/commands' },
      { text: 'aly-publish 命令', link: '/docs/publish' },
      { text: '服务端 API', link: '/docs/api' },
      { text: 'C# SDK API', link: '/docs/sdk/api' },
      { text: '多语言 SDK', link: '/docs/sdk/languages' }
    ]
  },
  {
    text: '帮助',
    items: [
      { text: '排障', link: '/docs/troubleshooting' },
      { text: 'FAQ', link: '/docs/faq' },
      { text: '文档版本', link: '/docs/versions' }
    ]
  },
  { text: '下载', link: '/docs/download' }
]

const enNav = [
  { text: 'Docs', link: '/en/docs/', activeMatch: '^/en/docs/$' },
  { text: 'Quick Start', link: '/en/docs/getting-started' },
  {
    text: 'Reference',
    items: [
      { text: 'aly-client commands', link: '/en/docs/commands' },
      { text: 'aly-publish commands', link: '/en/docs/publish' },
      { text: 'Server API', link: '/en/docs/api' },
      { text: 'C# SDK API', link: '/en/docs/sdk/api' },
      { text: 'Language SDKs', link: '/en/docs/sdk/languages' }
    ]
  },
  {
    text: 'Help',
    items: [
      { text: 'Troubleshooting', link: '/en/docs/troubleshooting' },
      { text: 'FAQ', link: '/en/docs/faq' },
      { text: 'Documentation versions', link: '/en/docs/versions' }
    ]
  },
  { text: 'Download', link: '/en/docs/download' }
]

const localSearchTranslations = {
  button: { buttonText: '搜索文档', buttonAriaLabel: '搜索文档' },
  modal: {
    noResultsText: '没有找到结果',
    resetButtonTitle: '清除查询条件',
    displayDetails: '显示详细列表',
    footer: {
      selectText: '选择',
      navigateText: '切换',
      closeText: '关闭'
    }
  }
}

export default defineConfig({
  title: 'Aly',
  base: '/aly/',
  srcExclude: ['scripts/**'],
  cleanUrls: true,
  appearance: 'dark',
  lastUpdated: true,
  sitemap: {
    hostname: siteUrl
  },

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/aly/logo.svg' }],
    ['meta', { name: 'theme-color', content: '#6366f1' }],
    ['meta', { name: 'author', content: 'lishuangquan1987' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:site_name', content: 'Aly' }],
    ['meta', { property: 'og:image', content: `${siteUrl}images/og-cover.png` }],
    ['meta', { property: 'og:image:width', content: '1200' }],
    ['meta', { property: 'og:image:height', content: '630' }],
    ['meta', { name: 'twitter:card', content: 'summary_large_image' }],
    ['meta', { name: 'twitter:image', content: `${siteUrl}images/og-cover.png` }]
  ],

  transformHead({ pageData }) {
    const isEn = pageData.relativePath.startsWith('en/')
    const fallbackTitle = isEn
      ? 'Aly — Lightweight Client Auto-Update System'
      : 'Aly — 轻量级客户端自动更新系统'
    const fallbackDescription = isEn
      ? 'Crash-safe atomic replacement, Windows XP through Linux, one JSON protocol for any host.'
      : '崩溃安全的原子替换、Windows XP 到 Linux、统一 JSON 协议，任何语言都能做宿主。'
    const title = pageData.title ? `${pageData.title} | Aly` : fallbackTitle
    const description = pageData.description || fallbackDescription
    return [
      ['meta', { property: 'og:title', content: title }],
      ['meta', { property: 'og:description', content: description }],
      ['meta', { name: 'twitter:title', content: title }],
      ['meta', { name: 'twitter:description', content: description }]
    ]
  },

  locales: {
    root: {
      label: '简体中文',
      lang: 'zh-CN',
      description:
        '轻量级客户端自动更新系统：崩溃安全的原子替换、XP 到 Linux 全平台覆盖、自更新闭环、148 条端到端用例守护。',
      themeConfig: {
        logo: '/logo.svg',
        nav: zhNav,
        sidebar: { '/docs/': zhSidebar },
        search: {
          provider: 'local',
          options: { translations: localSearchTranslations }
        },
        outline: { level: [2, 3], label: '本页目录' },
        docFooter: { prev: '上一篇', next: '下一篇' },
        lastUpdated: { text: '最后更新于' },
        returnToTopLabel: '回到顶部',
        sidebarMenuLabel: '菜单',
        darkModeSwitchLabel: '主题',
        lightModeSwitchTitle: '切换到浅色模式',
        darkModeSwitchTitle: '切换到深色模式',
        editLink: {
          pattern: `${repoUrl}/edit/master/website/:path`,
          text: '在 GitHub 编辑此页'
        },
        footer: {
          message: '基于 MIT 许可发布',
          copyright: 'Copyright © 2026 lishuangquan1987'
        }
      }
    },

    en: {
      label: 'English',
      lang: 'en-US',
      link: '/en/',
      description:
        'Lightweight client auto-update system: crash-safe atomic replacement, XP through Linux, self-update loop closed, guarded by 148 E2E cases.',
      themeConfig: {
        logo: '/logo.svg',
        nav: enNav,
        sidebar: { '/en/docs/': enSidebar },
        search: { provider: 'local' },
        outline: { level: [2, 3], label: 'On this page' },
        docFooter: { prev: 'Previous', next: 'Next' },
        lastUpdated: { text: 'Last updated' },
        returnToTopLabel: 'Return to top',
        sidebarMenuLabel: 'Menu',
        darkModeSwitchLabel: 'Appearance',
        lightModeSwitchTitle: 'Switch to light theme',
        darkModeSwitchTitle: 'Switch to dark theme',
        editLink: {
          pattern: `${repoUrl}/edit/master/website/:path`,
          text: 'Edit this page on GitHub'
        },
        footer: {
          message: 'Released under the MIT License.',
          copyright: 'Copyright © 2026 lishuangquan1987'
        }
      }
    }
  },

  themeConfig: {
    search: { provider: 'local' },
    socialLinks: [{ icon: 'github', link: repoUrl }]
  }
})