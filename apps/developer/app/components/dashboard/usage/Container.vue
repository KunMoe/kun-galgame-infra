<script setup lang="ts">
import { docsFaceLabel } from '~/constants/docs'
import { DEV_USAGE_DAY_OPTIONS } from '~/constants/dev'
import type { DevBreakdownRow, DevUsageSummary } from '~~/shared/types/dev'

useSeoMeta({ title: '用量', robots: 'noindex' })

const days = ref(7)

const { data, status } = await useApiFetch<DevUsageSummary>(
  () => `/dev/usage?days=${days.value}`
)

const summary = computed(() => data.value)
const daily = computed(() => summary.value?.daily ?? [])
const live = computed(() => summary.value?.live ?? [])
const liveUnavailable = computed(() => summary.value?.live_unavailable ?? false)

const fmt = (n: number) => n.toLocaleString()

const total = computed(() => summary.value?.total_count ?? 0)
const total4xx = computed(() => summary.value?.total_4xx ?? 0)
const total5xx = computed(() => summary.value?.total_5xx ?? 0)
const errors = computed(() => total4xx.value + total5xx.value)
const errorPct = computed(() =>
  total.value > 0 ? (errors.value / total.value) * 100 : 0
)

const byApp = computed<DevBreakdownRow[]>(() =>
  (summary.value?.by_app ?? []).map((row) => ({
    key: row.client_id,
    label: row.name,
    to: `/dashboard/apps/${row.client_id}`,
    count: row.count,
    status_4xx: row.status_4xx,
    status_5xx: row.status_5xx
  }))
)

const byFace = computed<DevBreakdownRow[]>(() =>
  (summary.value?.by_face ?? []).map((row) => ({
    key: row.face,
    label: docsFaceLabel(row.face),
    count: row.count,
    status_4xx: row.status_4xx,
    status_5xx: row.status_5xx
  }))
)

const peak = computed(() =>
  daily.value.reduce((best, d) => (d.count > best.count ? d : best), {
    day: '',
    count: 0,
    status_4xx: 0,
    status_5xx: 0
  })
)
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-foreground text-2xl font-bold">用量</h1>
        <p class="text-default-500 mt-1 text-sm">
          你名下所有应用的调用量汇总(按 UTC 日)。
        </p>
      </div>
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

    <div class="grid grid-cols-2 gap-4 md:grid-cols-4">
      <ChartStatTile
        label="总请求"
        :value="fmt(total)"
        :spark="daily.map((d) => d.count)"
        :hint="`最近 ${days} 天`"
      />
      <ChartStatTile
        label="错误率"
        :value="`${errorPct.toFixed(1)}%`"
        :meter="errorPct"
        :meter-color="
          errorPct >= 5 ? 'danger' : errorPct >= 1 ? 'warning' : 'success'
        "
        :hint="`${fmt(errors)} 次非 2xx/3xx`"
      />
      <ChartStatTile
        label="4xx"
        :value="fmt(total4xx)"
        :tone="total4xx > 0 ? 'warning' : 'muted'"
        hint="请求本身被拒"
      />
      <ChartStatTile
        label="5xx"
        :value="fmt(total5xx)"
        :tone="total5xx > 0 ? 'danger' : 'muted'"
        hint="服务端故障"
      />
    </div>

    <KunCard content-class="justify-start gap-0 items-stretch" class-name="p-6">
      <div class="mb-4 flex flex-wrap items-baseline justify-between gap-2">
        <h2 class="text-foreground text-base font-semibold">每日调用量</h2>
        <span class="text-default-400 text-xs">
          最近 {{ days }} 天 · 峰值
          {{ peak.day ? `${fmt(peak.count)}（${peak.day}）` : '—' }}
        </span>
      </div>
      <DashboardUsageTrend :days="daily" />
    </KunCard>

    <div>
      <div class="mb-3 flex items-center justify-between">
        <h2 class="text-foreground text-lg font-semibold">实时配额剩余</h2>
        <span class="text-default-400 text-xs">按 UTC 日 · 实时</span>
      </div>

      <KunCard v-if="liveUnavailable" content-class="p-10">
        <p class="text-default-400 text-center">
          实时配额暂不可用(计数后端不可达),稍后重试。
        </p>
      </KunCard>

      <DashboardUsageQuota v-else-if="live.length" :keys="live" />

      <KunCard v-else content-class="p-10">
        <p class="text-default-400 text-center">
          还没有活跃密钥。在应用详情页创建一把即可开始调用。
        </p>
      </KunCard>
    </div>

    <div>
      <h2 class="text-foreground mb-3 text-lg font-semibold">按应用</h2>
      <DashboardUsageBreakdown
        :rows="byApp"
        column-label="应用"
        empty-text="还没有应用产生调用。"
      />
    </div>

    <div>
      <h2 class="text-foreground mb-3 text-lg font-semibold">按接口</h2>
      <DashboardUsageBreakdown
        :rows="byFace"
        column-label="接口"
        empty-text="这段时间还没有接口被调用。"
      />
    </div>

    <p class="text-default-400 text-xs leading-relaxed">
      「实时配额剩余」直接读自执法计数器(与限流同源);上方历史用量按 UTC
      日累计,计量周期性落库,最新数据可能有几分钟延迟。
    </p>

    <p v-if="status === 'pending'" class="text-default-400 text-center text-xs">
      加载中…
    </p>
  </div>
</template>
