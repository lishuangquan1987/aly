import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: 'Aly',
  base: '/aly/',
  cleanUrls: true,
  lastUpdated: true,

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/aly/logo.svg' }],
    ['meta', { name: 'theme-color', content: '#5f67ee' }]
  ],

  locales: {
    root: {
      label: '简体中文',
      lang: 'zh-CN',
      description:
        '轻量级客户端自动更新系统：崩溃安全的原子替换、XP 到 Linux 全平台覆盖、自更新闭环、135 条端到端用例守护。',
      themeConfig: {
        nav: [
          { text: '介绍', link: '/intro', activeMatch: '^/intro$' },
          { text: '文档', link: '/docs/getting-started', activeMatch: '/docs/' }
        ],

        sidebar: {
          '/docs/': [
            {
              text: '入门',
              items: [{ text: '快速开始', link: '/docs/getting-started' }]
            },
            {
              text: '指南',
              items: [
                { text: '核心概念', link: '/docs/concepts' },
                { text: '系统架构', link: '/docs/architecture' }
              ]
            },
            {
              text: '参考',
              items: [
                { text: 'aly-client 命令速查', link: '/docs/commands' },
                { text: '服务端 API', link: '/docs/api' },
                { text: 'aly-publish 发布工具', link: '/docs/publish' },
                { text: 'C# SDK 集成', link: '/docs/sdk' }
              ]
            },
            {
              text: '进阶',
              items: [
                { text: '构建与跨平台', link: '/docs/build' },
                { text: '自更新设计', link: '/docs/self-update' },
                { text: '测试体系', link: '/docs/testing' }
              ]
            },
            {
              text: '其他',
              items: [{ text: 'FAQ', link: '/docs/faq' }]
            }
          ]
        },

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
    },

    en: {
      label: 'English',
      lang: 'en-US',
      link: '/en/',
      description:
        'Lightweight client auto-update system: crash-safe atomic replacement, XP through Linux, self-update loop closed, guarded by 135 E2E cases.',
      themeConfig: {
        nav: [
          { text: 'Introduction', link: '/en/intro', activeMatch: '^/en/intro$' },
          { text: 'Docs', link: '/en/docs/getting-started', activeMatch: '/en/docs/' }
        ],

        sidebar: {
          '/en/docs/': [
            {
              text: 'Getting Started',
              items: [{ text: 'Quick Start', link: '/en/docs/getting-started' }]
            },
            {
              text: 'Guide',
              items: [
                { text: 'Core Concepts', link: '/en/docs/concepts' },
                { text: 'Architecture', link: '/en/docs/architecture' }
              ]
            },
            {
              text: 'Reference',
              items: [
                { text: 'aly-client Commands', link: '/en/docs/commands' },
                { text: 'Server API', link: '/en/docs/api' },
                { text: 'aly-publish Tool', link: '/en/docs/publish' },
                { text: 'C# SDK', link: '/en/docs/sdk' }
              ]
            },
            {
              text: 'Advanced',
              items: [
                { text: 'Build & Cross-Platform', link: '/en/docs/build' },
                { text: 'Self-Update Design', link: '/en/docs/self-update' },
                { text: 'Testing', link: '/en/docs/testing' }
              ]
            },
            {
              text: 'Misc',
              items: [{ text: 'FAQ', link: '/en/docs/faq' }]
            }
          ]
        },

        search: { provider: 'local' },
        outline: { level: [2, 3], label: 'On this page' },

        footer: {
          message: 'Released under the MIT License.',
          copyright: 'Copyright © 2026 lishuangquan1987'
        }
      }
    }
  },

  themeConfig: {
    socialLinks: [{ icon: 'github', link: 'https://github.com/lishuangquan1987/aly' }]
  }
})
