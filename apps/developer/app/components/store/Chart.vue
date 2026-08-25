<script setup lang="ts">
import type { StoreUsageDay } from '~~/shared/types/store'

const props = defineProps<{ days: StoreUsageDay[] }>()

const max = computed(() => Math.max(1, ...props.days.map((d) => d.total)))

const barHeight = (value: number) => `${Math.round((value / max.value) * 100)}%`

const labelAt = (i: number) => {
  const n = props.days.length
  return i === 0 || i === n - 1 || i === Math.floor((n - 1) / 2)
}

const shortDay = (day: string) => day.slice(5)
</script>

<template>
  <div v-if="days.length">
    <div class="flex h-48 items-end gap-1">
      <div
        v-for="d in days"
        :key="d.day"
        class="group relative flex h-full flex-1 items-end"
      >
        <div
          class="relative min-h-[3px] w-full rounded-t transition-colors"
          :class="d.total > 0 ? 'bg-primary-200' : 'bg-default-200'"
          :style="{ height: barHeight(d.total) }"
        >
          <div
            class="absolute inset-x-0 bottom-0 rounded-t bg-primary transition-colors"
            :style="{ height: barHeight(d.uniques) }"
          />
        </div>
        <div
          class="pointer-events-none absolute -top-14 left-1/2 z-10 hidden -translate-x-1/2 whitespace-nowrap rounded-md bg-foreground px-2 py-1 text-xs text-background shadow-sm group-hover:block"
        >
          <p class="font-mono">{{ d.day }}</p>
          <p>
            去重 <span class="font-semibold">{{ d.uniques.toLocaleString() }}</span>
            / 总 {{ d.total.toLocaleString() }}
          </p>
        </div>
      </div>
    </div>
    <div class="mt-2 flex gap-1">
      <div
        v-for="(d, i) in days"
        :key="d.day"
        class="flex-1 text-center text-[10px] text-default-400"
      >
        <span v-if="labelAt(i)" class="font-mono">{{ shortDay(d.day) }}</span>
      </div>
    </div>
  </div>

  <div
    v-else
    class="flex h-48 items-center justify-center text-sm text-default-400"
  >
    暂无数据
  </div>
</template>
