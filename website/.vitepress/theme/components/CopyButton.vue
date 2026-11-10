<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useData } from 'vitepress'

const props = withDefaults(
  defineProps<{
    text: string
    label?: string
    copiedLabel?: string
  }>(),
  { label: '', copiedLabel: '' }
)

const { lang } = useData()
const isEn = computed(() => lang.value === 'en-US')
const copied = ref(false)
const failed = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined

const buttonLabel = computed(() => {
  if (failed.value) return isEn.value ? 'Copy failed' : '复制失败'
  if (copied.value) return props.copiedLabel || (isEn.value ? 'Copied' : '已复制')
  return props.label || (isEn.value ? 'Copy' : '复制')
})

async function copy() {
  failed.value = false
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(props.text)
    } else {
      const area = document.createElement('textarea')
      area.value = props.text
      area.setAttribute('readonly', '')
      area.style.position = 'fixed'
      area.style.opacity = '0'
      document.body.appendChild(area)
      area.select()
      const ok = document.execCommand('copy')
      document.body.removeChild(area)
      if (!ok) throw new Error('execCommand copy failed')
    }
    copied.value = true
    clearTimeout(timer)
    timer = setTimeout(() => {
      copied.value = false
    }, 1600)
  } catch {
    failed.value = true
    clearTimeout(timer)
    timer = setTimeout(() => {
      failed.value = false
    }, 2200)
  }
}

onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <button type="button" class="aly-copy-btn" :class="{ 'is-done': copied, 'is-failed': failed }" @click="copy">
    {{ buttonLabel }}
  </button>
</template>