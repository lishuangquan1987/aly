import type { Theme } from 'vitepress'
import DefaultTheme from 'vitepress/theme'
import Layout from './Layout.vue'
import CopyButton from './components/CopyButton.vue'
import DocHub from './components/DocHub.vue'
import QuickStartWizard from './components/QuickStartWizard.vue'
import ScreenshotFrame from './components/ScreenshotFrame.vue'
import './styles/custom.css'
import './styles/home.css'

export default {
  extends: DefaultTheme,
  Layout,
  enhanceApp({ app }) {
    app.component('CopyButton', CopyButton)
    app.component('DocHub', DocHub)
    app.component('QuickStartWizard', QuickStartWizard)
    app.component('ScreenshotFrame', ScreenshotFrame)
  }
} satisfies Theme