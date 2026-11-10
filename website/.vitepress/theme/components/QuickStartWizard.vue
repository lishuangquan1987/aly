<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useData, withBase } from 'vitepress'
import {
  buildQuickStartSteps,
  quickStartCopy,
  type QuickStartArch,
  type QuickStartGoal,
  type QuickStartLang,
  type QuickStartOs
} from '../data/quickstart'

const { lang } = useData()
const isEn = computed(() => lang.value === 'en-US')
const copy = computed(() => quickStartCopy[isEn.value ? 'en' : 'zh'])
const prefix = computed(() => (isEn.value ? '/en' : ''))

const goal = ref<QuickStartGoal>('publish')
const os = ref<QuickStartOs>('windows')
const arch = ref<QuickStartArch>('amd64')
const hostLang = ref<QuickStartLang>('csharp')

const goals = computed(() =>
  (['publish', 'integrate', 'server'] as const).map((id) => ({ id, label: copy.value.goals[id] }))
)
const osList = computed<QuickStartOs[]>(() =>
  goal.value === 'integrate' ? ['windows', 'linux'] : ['windows', 'linux', 'macos']
)
const archList = computed(() => [
  { id: 'amd64' as const, label: copy.value.archs.amd64 },
  { id: 'arm64' as const, label: copy.value.archs.arm64 }
])
const showArch = computed(
  () => os.value !== 'windows' && (goal.value !== 'integrate' || os.value === 'linux')
)
const showLang = computed(() => goal.value === 'integrate')
const steps = computed(() =>
  buildQuickStartSteps(isEn.value, goal.value, os.value, arch.value, hostLang.value)
)
const docLink = computed(() => {
  const key = goal.value === 'integrate' ? hostLang.value : goal.value
  return copy.value.docLink[key]
})

watch(goal, () => {
  if (!osList.value.includes(os.value)) os.value = 'windows'
})
watch(os, () => {
  if (os.value === 'windows') arch.value = 'amd64'
})

function codeLang(code: string): string {
  const trimmed = code.trim()
  if (trimmed.startsWith('{')) return 'json'
  if (trimmed.includes('using AlyClient') || trimmed.includes('var ui =')) return 'csharp'
  if (trimmed.startsWith('Invoke-WebRequest')) return 'powershell'
  return 'shell'
}
</script>

<template>
  <section class="aly-qs">
    <div class="aly-qs-head">
      <span class="aly-kicker">{{ copy.kicker }}</span>
      <h2>{{ copy.title }}</h2>
      <p>{{ copy.lead }}</p>
    </div>

    <div class="aly-qs-controls">
      <div class="aly-qs-control">
        <span class="aly-qs-label">{{ copy.goalLabel }}</span>
        <div class="aly-qs-options" role="group" :aria-label="copy.goalLabel">
          <button
            v-for="item in goals"
            :key="item.id"
            type="button"
            class="aly-qs-option"
            :class="{ 'is-active': goal === item.id }"
            :aria-pressed="goal === item.id"
            @click="goal = item.id"
          >
            {{ item.label }}
          </button>
        </div>
      </div>

      <div class="aly-qs-control">
        <span class="aly-qs-label">{{ copy.osLabel }}</span>
        <div class="aly-qs-options" role="group" :aria-label="copy.osLabel">
          <button
            v-for="item in osList"
            :key="item"
            type="button"
            class="aly-qs-option"
            :class="{ 'is-active': os === item }"
            :aria-pressed="os === item"
            @click="os = item"
          >
            {{ copy.oss[item] }}
          </button>
        </div>
      </div>

      <div v-if="showArch" class="aly-qs-control">
        <span class="aly-qs-label">{{ copy.archLabel }}</span>
        <div class="aly-qs-options" role="group" :aria-label="copy.archLabel">
          <button
            v-for="item in archList"
            :key="item.id"
            type="button"
            class="aly-qs-option"
            :class="{ 'is-active': arch === item.id }"
            :aria-pressed="arch === item.id"
            @click="arch = item.id"
          >
            {{ item.label }}
          </button>
        </div>
      </div>

      <div v-if="showLang" class="aly-qs-control">
        <span class="aly-qs-label">{{ copy.langLabel }}</span>
        <div class="aly-qs-options" role="group" :aria-label="copy.langLabel">
          <button
            type="button"
            class="aly-qs-option"
            :class="{ 'is-active': hostLang === 'csharp' }"
            :aria-pressed="hostLang === 'csharp'"
            @click="hostLang = 'csharp'"
          >
            {{ copy.langs.csharp }}
          </button>
          <button
            type="button"
            class="aly-qs-option"
            :class="{ 'is-active': hostLang === 'cli' }"
            :aria-pressed="hostLang === 'cli'"
            @click="hostLang = 'cli'"
          >
            {{ copy.langs.cli }}
          </button>
        </div>
      </div>

      <p v-if="os === 'windows' && goal !== 'integrate'" class="aly-qs-hint">
        {{ isEn ? 'Windows builds are amd64 only.' : 'Windows 仅提供 amd64 版本。' }}
      </p>
    </div>

    <ol class="aly-qs-steps" aria-live="polite">
      <li v-for="(step, index) in steps" :key="`${goal}-${os}-${hostLang}-${index}`">
        <div class="aly-qs-step-head">
          <span class="aly-qs-num">{{ String(index + 1).padStart(2, '0') }}</span>
          <h3>{{ step.title }}</h3>
        </div>
        <p v-if="step.body" class="aly-qs-body">{{ step.body }}</p>
        <div v-if="step.code" class="aly-code">
          <div class="aly-code-head">
            <span class="aly-code-lang">{{ codeLang(step.code) }}</span>
            <CopyButton :text="step.code" />
          </div>
          <pre><code>{{ step.code }}</code></pre>
        </div>
        <p v-if="step.note" class="aly-qs-note">{{ step.note }}</p>
        <a v-if="step.link" class="aly-qs-link" :href="withBase(prefix + step.link.to)">
          {{ step.link.text }} <span aria-hidden="true">→</span>
        </a>
      </li>
    </ol>

    <div class="aly-qs-foot">
      <a :href="withBase(prefix + docLink)">
        {{ isEn ? 'Open the full reference for this path' : '打开这条路径的完整参考' }}
        <span aria-hidden="true">→</span>
      </a>
      <span>{{ isEn ? 'Every option below is copyable.' : '下方每一步都可以直接复制。' }}</span>
    </div>
  </section>
</template>