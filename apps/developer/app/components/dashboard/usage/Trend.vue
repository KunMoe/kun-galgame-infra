<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import { CHART_BAR_CATEGORY_GAP, CHART_STACK_GAP } from '~/constants/chart'
import type { DevUsageDay } from '~~/shared/types/dev'

const props = defineProps<{ days: DevUsageDay[] }>()

const theme = useChartTheme()

const average = computed(() => {
  if (!props.days.length) return 0
  const sum = props.days.reduce((acc, d) => acc + d.count, 0)
  return Math.round(sum / props.days.length)
})

const ok = (d: DevUsageDay) =>
  Math.max(0, d.count - d.status_4xx - d.status_5xx)

const option = computed<EChartsOption>(() => {
  const t = theme.value
  const border = { borderColor: t.surface, borderWidth: CHART_STACK_GAP }
  const bar = (
    name: string,
    color: string,
    pick: (d: DevUsageDay) => number
  ) => ({
    type: 'bar' as const,
    name,
    stack: 'requests',
    barMaxWidth: 26,
    barCategoryGap: CHART_BAR_CATEGORY_GAP,
    data: props.days.map(pick),
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
    grid: chartGrid({ top: 36 }),
    tooltip: {
      ...(chartBase(t).tooltip as object),
      formatter: (params: unknown) => {
        const rows = params as { dataIndex: number }[]
        const d = props.days[rows[0]!.dataIndex]
        if (!d) return ''
        return [
          `<span style="font-family:monospace">${d.day}</span>`,
          `请求 <b>${d.count.toLocaleString()}</b>`,
          `4xx ${d.status_4xx.toLocaleString()} · 5xx ${d.status_5xx.toLocaleString()}`
        ].join('<br/>')
      }
    },
    xAxis: {
      ...categoryAxis(t),
      data: props.days.map((d) => d.day.slice(5))
    },
    yAxis: valueAxis(t),
    series: [
      {
        ...bar('成功', t.accent, ok),
        markLine: average.value
          ? {
              silent: true,
              symbol: 'none',
              lineStyle: { color: t.ink, type: 'dashed', width: 1 },
              label: {
                position: 'insideEndTop',
                color: t.ink,
                fontSize: 11,
                backgroundColor: t.surface,
                padding: [2, 4],
                borderRadius: 3,
                formatter: `日均 ${average.value.toLocaleString()}`
              },
              data: [{ yAxis: average.value }]
            }
          : undefined
      },
      bar('客户端错误 4xx', t.warning, (d) => d.status_4xx),
      bar('服务端错误 5xx', t.danger, (d) => d.status_5xx)
    ]
  }
})
</script>

<template>
  <ChartFrame
    :option="option"
    :height="300"
    :empty="!days.length"
    empty-text="这段时间还没有调用"
  />
</template>
