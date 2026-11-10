<script setup lang="ts">
import { withBase } from 'vitepress'

const props = withDefaults(
  defineProps<{
    src: string
    alt: string
    caption?: string
    wide?: boolean
  }>(),
  { caption: '', wide: false }
)
</script>

<template>
  <figure class="aly-shot" :class="{ 'is-wide': props.wide }">
    <a :href="withBase(props.src)" target="_blank" rel="noopener">
      <img :src="withBase(props.src)" :alt="props.alt" loading="lazy" decoding="async" />
    </a>
    <figcaption v-if="props.caption">{{ props.caption }}</figcaption>
  </figure>
</template>