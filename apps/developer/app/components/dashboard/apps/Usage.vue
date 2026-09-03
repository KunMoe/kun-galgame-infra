<script setup lang="ts">
import { docsFaceLabel } from '~/constants/docs'
import { DEV_USAGE_DAY_OPTIONS } from '~/constants/dev'
import type {
  DevBreakdownRow,
  DevUsageDay,
  DevUsageDayFace
} from '~~/shared/types/dev'

const props = defineProps<{ clientId: string }>()

const days = ref(7)

const { data } = await useApiFetch<DevUsageDayFace[]>(
  () => `/dev/apps/${props.clientId}/usage?days=${days.value}`
)

const rows = computed(() => data.value ?? [])

// The endpoint returns one row per (day, face); the trend needs one per day and
// the breakdown one per face, so both are folded here rather than asked for.
const daily = computed<DevUsageDay[]>(() => {
  const byDay = new Map<string, DevUsageDay>()
  for (const row of rows.value) {
    const seen = byDay.get(row.day) ?? {
      day: row.day,
      count: 0,
      status_4xx: 0,
      status_5xx: 0
    }
    seen.count += row.count
    seen.status_4xx += row.status_4xx
    seen.status_5xx += row.status_5xx
    byDay.set(row.day, seen)
  }
  return [...byDay.values()].sort((a, b) => a.day.localeCompare(b.day))
})

const byFace = computed<DevBreakdownRow[]>(() => {
  const byKey = new Map<string, DevBreakdownRow>()
  for (const row of rows.value) {
    const seen = byKey.get(row.face) ?? {
      key: row.face,
      label: docsFaceLabel(row.face),
      count: 0,
      status_4xx: 0,
      status_5xx: 0
    }
    seen.count += row.count
    seen.status_4xx += row.status_4xx
    seen.status_5xx += row.status_5xx
    byKey.set(row.face, seen)
  }
  return [...byKey.values()].sort((a, b) => b.count - a.count)
})

const totals = computed(() =>
  daily.value.reduce(
    (acc, d) => ({
      count: acc.count + d.count,
      errors: acc.errors + d.status_4xx + d.status_5xx
    }),
    { count: 0, errors: 0 }
  )
)

const fmt = (n: number) => n.toLocaleString()
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-foreground text-lg font-semibold">用量</h2>
      <div class="border-default-200 flex rounded-lg border p-0.5">
        <button
          v-for="opt in DEV_USAGE_DAY_OPTIONS"
          :key="opt"
          type="button"
          class="rounded-md px-3 py-1 text-sm font-medium transition-colors"
          :class="
            days === opt
              ? 'bg-primary-50 text-primary'
              : 'text-default-500 hover:text-foreground'
          "
          @click="days = opt"
        >
          {{ opt }} 天
        </button>
      </div>
    </div>

    <KunCard content-class="justify-start gap-0 items-stretch" class-name="p-6">
      <div class="mb-4 flex flex-wrap items-baseline justify-between gap-2">
        <h3 class="text-foreground text-base font-semibold">每日调用量</h3>
        <span class="text-default-400 text-xs">
          {{ fmt(totals.count) }} 次请求 · {{ fmt(totals.errors) }} 次错误
        </span>
      </div>
      <DashboardUsageTrend :days="daily" />
    </KunCard>

    <DashboardUsageBreakdown
      :rows="byFace"
      column-label="接口"
      empty-text="这段时间该应用还没有调用记录。"
    />
  </div>
</template>
