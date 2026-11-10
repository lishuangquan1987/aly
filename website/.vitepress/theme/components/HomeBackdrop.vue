<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

const root = ref<HTMLElement | null>(null)
let frame = 0

function handlePointerMove(event: PointerEvent) {
  if (!root.value) return
  cancelAnimationFrame(frame)
  frame = requestAnimationFrame(() => {
    const rect = root.value?.getBoundingClientRect()
    if (!rect) return
    const x = ((event.clientX - rect.left) / rect.width) * 100
    const y = ((event.clientY - rect.top) / rect.height) * 100
    root.value?.style.setProperty('--aly-pointer-x', `${x}%`)
    root.value?.style.setProperty('--aly-pointer-y', `${y}%`)
  })
}

onMounted(() => {
  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const coarse = window.matchMedia('(pointer: coarse)').matches
  if (reduced || coarse) return
  window.addEventListener('pointermove', handlePointerMove, { passive: true })
})

onBeforeUnmount(() => {
  cancelAnimationFrame(frame)
  window.removeEventListener('pointermove', handlePointerMove)
})
</script>

<template>
  <div ref="root" class="aly-backdrop" aria-hidden="true">
    <div class="aly-aurora aly-aurora-a" />
    <div class="aly-aurora aly-aurora-b" />
    <div class="aly-aurora aly-aurora-c" />
    <div class="aly-grid" />
    <div class="aly-spotlight" />
  </div>
</template>