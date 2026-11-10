<script setup lang="ts">
import DefaultTheme from 'vitepress/theme'
import HomeBackdrop from './components/HomeBackdrop.vue'
import HomeCTA from './components/HomeCTA.vue'
import HomeFlow from './components/HomeFlow.vue'
import HomeRoles from './components/HomeRoles.vue'
import HomeShowcase from './components/HomeShowcase.vue'
import HeroTerminal from './components/HeroTerminal.vue'
import DocBreadcrumb from './components/DocBreadcrumb.vue'

const { Layout } = DefaultTheme
</script>

<template>
  <Layout>
    <template #doc-before>
      <DocBreadcrumb />
    </template>

    <template #home-hero-before>
      <HomeBackdrop />
    </template>

    <template #home-hero-image>
      <HeroTerminal />
    </template>

    <template #home-hero-after>
      <HomeRoles />
    </template>

    <template #home-features-before>
      <HomeShowcase />
    </template>

    <template #home-features-after>
      <HomeFlow />
      <HomeCTA />
    </template>
  </Layout>
</template>