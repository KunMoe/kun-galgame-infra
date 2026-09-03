<script setup lang="ts">
import { cn } from '@kungal/ui-core'

const props = withDefaults(
  defineProps<{
    label: string
    /** A number is shown compact (万/亿) with the exact figure on hover. */
    value: string | number
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

const display = computed(() =>
  typeof props.value === 'number'
    ? formatCompactNumber(props.value)
    : props.value
)

const exact = computed(() =>
  typeof props.value === 'number' ? props.value.toLocaleString() : ''
)

const isCompacted = computed(
  () => !!exact.value && exact.value !== display.value
)

const width = computed(() =>
  props.meter === undefined
    ? '0%'
    : `${Math.max(0, Math.min(100, props.meter))}%`
)
</script>

<template>
  <div
    class="border-default-200 bg-content1 @container rounded-xl border px-5 py-4"
  >
    <p class="text-default-400 text-xs">{{ label }}</p>
    <div class="mt-1 flex items-end justify-between gap-3">
      <KunTooltip
        v-if="isCompacted"
        :text="exact"
        position="top"
        class-name="min-w-0"
      >
        <p :class="cn('truncate text-2xl font-bold', TONE[tone])">
          {{ display }}
        </p>
      </KunTooltip>
      <p v-else :class="cn('truncate text-2xl font-bold', TONE[tone])">
        {{ display }}
      </p>
      <!--
        The spark yields its width to the number and drops out entirely once
        there is no room left for it. 9.5rem is the tile's CONTENT box, not its
        228px border box - a container query measures the content box, so a
        threshold picked off the visible tile width hides the spark on desktop.
      -->
      <ChartSparkline
        v-if="spark?.length"
        :values="spark"
        class="mb-1 w-full max-w-24 min-w-0 flex-1 @max-[9.5rem]:hidden"
      />
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
