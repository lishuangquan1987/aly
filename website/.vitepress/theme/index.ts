import type { Theme } from 'vitepress'
import DefaultTheme from 'vitepress/theme'
import Layout from './Layout.vue'
import DocHub from './components/DocHub.vue'
import ScreenshotFrame from './components/ScreenshotFrame.vue'
import ReleaseDownloads from './components/ReleaseDownloads.vue'
import ReleaseLink from './components/ReleaseLink.vue'
import ReleaseVersion from './components/ReleaseVersion.vue'
import './styles/custom.css'
import './styles/home.css'

export default {
  extends: DefaultTheme,
  Layout,
  enhanceApp({ app }) {
    app.component('DocHub', DocHub)
    app.component('ScreenshotFrame', ScreenshotFrame)
    app.component('ReleaseDownloads', ReleaseDownloads)
    app.component('ReleaseLink', ReleaseLink)
    app.component('ReleaseVersion', ReleaseVersion)
  }
} satisfies Theme