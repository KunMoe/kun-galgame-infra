<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import type { StoreUsageDay } from '~~/shared/types/store'

const props = defineProps<{ days: StoreUsageDay[] }>()

const theme = useChartTheme()

const NAMES = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']

// JST days arrive as YYYY-MM-DD. Parsing them as UTC keeps the weekday the
// string names — `new Date('2026-09-03')` is already UTC midnight, but going
// through the parts makes that independent of the runtime's local zone.
const weekdayOf = (day: string) => {
  const [y, m, d] = day.split('-').map(Number)
  return (new Date(Date.UTC(y!, m! - 1, d!)).getUTCDay() + 6) % 7
}

const buckets = computed(() => {
  const sum = Array.from({ length: 7 }, () => 0)
  const count = Array.from({ length: 7 }, () => 0)
  for (const d of props.days) {
    const i = weekdayOf(d.day)
    sum[i]! += d.uniques
    count[i]! += 1
  }
  return sum.map((s, i) => (count[i] ? Math.round(s / count[i]!) : 0))
})

const option = computed<EChartsOption>(() => {
  const t = theme.value
  return {
    ...chartBase(t),
    tooltip: {
      ...(chartBase(t).tooltip as object),
      formatter: (params: unknown) => {
        const [row] = params as { dataIndex: number; data: number }[]
        return `${NAMES[row!.dataIndex]}<br/>日均去重 <b>${row!.data.toLocaleString()}</b>`
      }
    },
    xAxis: { ...categoryAxis(t), data: NAMES },
    yAxis: valueAxis(t),
    series: [
      {
        type: 'bar',
        name: '日均去重点击',
        barMaxWidth: 34,
        data: buckets.value,
        itemStyle: { color: t.accent, borderRadius: [4, 4, 0, 0] }
      }
    ]
  }
})
</script>

<template>
  <ChartFrame
    :option="option"
    :height="200"
    :empty="!days.length"
    empty-text="这段时间还没有点击"
  />
</template>
