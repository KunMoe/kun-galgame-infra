<script setup lang="ts">
import { cn } from '@kungal/ui-core'

const props = withDefaults(
  defineProps<{
    label: string
    value: string
    hint?: string
    /** 0–100. Renders a meter under the value instead of a sparkline. */
    meter?: number
    meterColor?: 'primary' | 'success' | 'warning' | 'danger'
    spark?: number[]
    tone?: 'foreground' | 'warning' | 'danger' | 'muted'
  }>(),
  {
    tone: 'foreground',
    meterColor: 'primary',
    hint: '',
    meter: undefined,
    spark: undefined
  }
)

const TONE = {
  foreground: 'text-foreground',
  warning: 'text-warning',
  danger: 'text-danger',
  muted: 'text-default-400'
} as const

const METER = {
  primary: 'bg-primary',
  success: 'bg-success',
  warning: 'bg-warning',
  danger: 'bg-danger'
} as const

const width = computed(() =>
  props.meter === undefined
    ? '0%'
    : `${Math.max(0, Math.min(100, props.meter))}%`
)
</script>

<template>
  <div class="border-default-200 bg-content1 rounded-xl border px-5 py-4">
    <p class="text-default-400 text-xs">{{ label }}</p>
    <div class="mt-1 flex items-end justify-between gap-3">
      <p :class="cn('text-2xl font-bold', TONE[tone])">{{ value }}</p>
      <ChartSparkline v-if="spark?.length" :values="spark" class="mb-1" />
    </div>
    <div
      v-if="meter !== undefined"
      class="bg-default-200 mt-2.5 h-1.5 w-full overflow-hidden rounded-full"
    >
      <div
        :class="cn('h-full rounded-full transition-all', METER[meterColor])"
        :style="{ width }"
      />
    </div>
    <p v-if="hint" class="text-default-400 mt-2 text-xs">{{ hint }}</p>
  </div>
</template>
