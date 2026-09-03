<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import { CHART_BAR_CATEGORY_GAP, CHART_STACK_GAP } from '~/constants/chart'
import type { DevBreakdownRow } from '~~/shared/types/dev'

const props = defineProps<{
  rows: DevBreakdownRow[]
  columnLabel: string
  emptyText: string
}>()

const theme = useChartTheme()

const fmt = (n: number) => n.toLocaleString()

const ok = (row: DevBreakdownRow) =>
  Math.max(0, row.count - row.status_4xx - row.status_5xx)

// ECharts stacks a horizontal bar bottom-up, so the ranked list has to be
// reversed for the busiest row to land at the top of the plot.
const plotted = computed(() => [...props.rows].reverse())

const option = computed<EChartsOption>(() => {
  const t = theme.value
  const border = { borderColor: t.surface, borderWidth: CHART_STACK_GAP }
  const bar = (
    name: string,
    color: string,
    pick: (row: DevBreakdownRow) => number
  ) => ({
    type: 'bar' as const,
    name,
    stack: 'requests',
    barMaxWidth: 20,
    barCategoryGap: CHART_BAR_CATEGORY_GAP,
    data: plotted.value.map(pick),
    itemStyle: { color, ...border }
  })

  return {
    ...chartBase(t),
    legend: {
      data: ['成功', '客户端错误 4xx', '服务端错误 5xx'],
      top: 0,
      left: 0,
      icon: 'roundRect',
      itemWidth: 10,
      itemHeight: 10,
      itemGap: 18,
      textStyle: { color: t.ink, fontSize: 12 }
    },
    grid: chartGrid({ top: 36, right: 16 }),
    tooltip: {
      ...(chartBase(t).tooltip as object),
      formatter: (params: unknown) => {
        const hovered = params as { dataIndex: number }[]
        const row = plotted.value[hovered[0]!.dataIndex]
        if (!row) return ''
        return [
          row.label,
          `请求 <b>${fmt(row.count)}</b>`,
          `4xx ${fmt(row.status_4xx)} · 5xx ${fmt(row.status_5xx)}`
        ].join('<br/>')
      }
    },
    xAxis: valueAxis(t),
    yAxis: {
      type: 'category',
      data: plotted.value.map((row) => row.label),
      axisTick: { show: false },
      axisLine: { show: false },
      axisLabel: {
        color: t.ink,
        fontSize: 12,
        width: 140,
        overflow: 'truncate'
      }
    },
    series: [
      bar('成功', t.accent, ok),
      bar('客户端错误 4xx', t.warning, (row) => row.status_4xx),
      bar('服务端错误 5xx', t.danger, (row) => row.status_5xx)
    ]
  }
})
</script>

<template>
  <KunCard
    v-if="rows.length"
    content-class="justify-start gap-0 items-stretch"
    class-name="p-6"
  >
    <ChartFrame :option="option" :height="rows.length * 42 + 56" />

    <div class="mt-4 overflow-x-auto">
      <table class="w-full min-w-[32rem] text-sm">
        <thead>
          <tr class="border-default-200 text-default-400 border-b text-left">
            <th class="px-4 py-2 font-medium">{{ columnLabel }}</th>
            <th class="px-4 py-2 text-right font-medium">请求数</th>
            <th class="px-4 py-2 text-right font-medium">4xx</th>
            <th class="px-4 py-2 text-right font-medium">5xx</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(row, i) in rows"
            :key="row.key"
            class="border-default-100 border-b"
            :class="i === rows.length - 1 && 'border-b-0'"
          >
            <td class="px-4 py-2">
              <NuxtLink
                v-if="row.to"
                :to="row.to"
                class="text-foreground hover:text-primary font-medium hover:underline"
              >
                {{ row.label }}
              </NuxtLink>
              <KunChip v-else color="default" variant="flat" size="xs">
                {{ row.label }}
              </KunChip>
            </td>
            <td
              class="text-foreground px-4 py-2 text-right font-medium tabular-nums"
            >
              {{ fmt(row.count) }}
            </td>
            <td
              class="px-4 py-2 text-right tabular-nums"
              :class="row.status_4xx > 0 ? 'text-warning' : 'text-default-400'"
            >
              {{ fmt(row.status_4xx) }}
            </td>
            <td
              class="px-4 py-2 text-right tabular-nums"
              :class="row.status_5xx > 0 ? 'text-danger' : 'text-default-400'"
            >
              {{ fmt(row.status_5xx) }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </KunCard>

  <KunCard v-else content-class="p-10">
    <p class="text-default-400 text-center">{{ emptyText }}</p>
  </KunCard>
</template>
