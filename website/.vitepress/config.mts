import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  lang: 'zh-CN',
  title: 'Aly',
  description:
    '轻量级客户端自动更新系统：崩溃安全的原子替换、XP 到 Linux 全平台覆盖、自更新闭环、135 条端到端用例守护。',
  base: '/aly/',
  cleanUrls: true,
  lastUpdated: true,

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/aly/logo.svg' }],
    ['meta', { name: 'theme-color', content: '#5f67ee' }]
  ],

  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    nav: [
      { text: '指南', link: '/guide/overview', activeMatch: '/guide/' },
      { text: '参考', link: '/reference/commands', activeMatch: '/reference/' },
      { text: '进阶', link: '/advanced/build', activeMatch: '/advanced/' },
      { text: 'FAQ', link: '/faq' }
    ],

    sidebar: {
      '/guide/': [
        {
          text: '指南',
          items: [
            { text: '项目简介', link: '/guide/overview' },
            { text: '快速开始', link: '/guide/getting-started' },
            { text: '核心概念', link: '/guide/concepts' },
            { text: '系统架构', link: '/guide/architecture' }
          ]
        }
      ],
      '/reference/': [
        {
          text: '参考',
          items: [
            { text: 'aly-client 命令速查', link: '/reference/commands' },
            { text: '服务端 API', link: '/reference/api' },
            { text: 'aly-publish 发布工具', link: '/reference/publish' },
            { text: 'C# SDK 集成', link: '/reference/sdk' }
          ]
        }
      ],
      '/advanced/': [
        {
          text: '进阶',
          items: [
            { text: '构建与跨平台', link: '/advanced/build' },
            { text: '自更新设计', link: '/advanced/self-update' },
            { text: '测试体系', link: '/advanced/testing' }
          ]
        }
      ]
    },

    socialLinks: [{ icon: 'github', link: 'https://github.com/lishuangquan1987/aly' }],

    search: {
      provider: 'local',
      options: {
        translations: {
          button: { buttonText: '搜索文档', buttonAriaLabel: '搜索文档' },
          modal: {
            noResultsText: '没有找到结果',
            resetButtonTitle: '清除查询条件',
            displayDetails: '显示详细列表',
            footer: { selectText: '选择', navigateText: '切换', closeText: '关闭' }
          }
        }
      }
    },

    outline: { level: [2, 3], label: '本页目录' },
    docFooter: { prev: '上一篇', next: '下一篇' },
    lastUpdated: { text: '最后更新于' },
    returnToTopLabel: '回到顶部',
    sidebarMenuLabel: '菜单',
    darkModeSwitchLabel: '主题',
    lightModeSwitchTitle: '切换到浅色模式',
    darkModeSwitchTitle: '切换到深色模式',

    footer: {
      message: '基于 MIT 许可发布',
      copyright: 'Copyright © 2026 lishuangquan1987'
    }
  }
})
