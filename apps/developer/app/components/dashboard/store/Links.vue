<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import {
  STORE_LINK_KIND_COLORS,
  STORE_LINK_KIND_FILTERS,
  STORE_LINK_KIND_LABELS,
  STORE_LINK_PAGE_SIZE,
  STORE_LINK_SORTS,
  STORE_TOP_LINKS,
  type StoreLinkSort
} from '~/constants/store'
import type { StoreUsageLink } from '~~/shared/types/store'

const props = defineProps<{ rows: StoreUsageLink[] }>()

const theme = useChartTheme()

const query = ref('')
const kind = ref<string>('all')
const sort = ref<StoreLinkSort>('uniques')
const page = ref(1)

const fmt = (n: number) => n.toLocaleString()

const target = (row: StoreUsageLink) =>
  row.product_id ?? (row.campaign_id === null ? '—' : `#${row.campaign_id}`)

const rowKey = (row: StoreUsageLink) =>
  `${row.client_id}-${row.kind}-${row.product_id ?? row.campaign_id}`

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  const list = props.rows.filter((row) => {
    if (kind.value !== 'all' && row.kind !== kind.value) return false
    if (!q) return true
    return (
      target(row).toLowerCase().includes(q) ||
      row.app_name.toLowerCase().includes(q)
    )
  })
  const by = sort.value
  return list.sort((a, b) =>
    by === 'target'
      ? target(a).localeCompare(target(b))
      : b[by] - a[by] || a.client_id.localeCompare(b.client_id)
  )
})

const totalPage = computed(() =>
  Math.max(1, Math.ceil(filtered.value.length / STORE_LINK_PAGE_SIZE))
)

// A narrower filter can leave the reader stranded past the last page.
watch([query, kind, sort], () => {
  page.value = 1
})
watch(totalPage, (n) => {
  if (page.value > n) page.value = n
})

const paged = computed(() =>
  filtered.value.slice(
    (page.value - 1) * STORE_LINK_PAGE_SIZE,
    page.value * STORE_LINK_PAGE_SIZE
  )
)

const peak = computed(() =>
  Math.max(1, ...filtered.value.map((row) => row.uniques))
)

const share = (row: StoreUsageLink) => `${(row.uniques / peak.value) * 100}%`

const top = computed(() =>
  [...filtered.value]
    .sort((a, b) => b.uniques - a.uniques)
    .slice(0, STORE_TOP_LINKS)
    .reverse()
)

const option = computed<EChartsOption>(() => {
  const t = theme.value
  return {
    ...chartBase(t),
    grid: chartGrid({ right: 24 }),
    tooltip: {
      ...(chartBase(t).tooltip as object),
      formatter: (params: unknown) => {
        const rows = params as { dataIndex: number }[]
        const row = top.value[rows[0]!.dataIndex]
        if (!row) return ''
        return [
          `<span style="font-family:monospace">${target(row)}</span>`,
          `${row.app_name} · ${STORE_LINK_KIND_LABELS[row.kind]}`,
          `去重 <b>${fmt(row.uniques)}</b> / 总 ${fmt(row.total)}`
        ].join('<br/>')
      }
    },
    xAxis: valueAxis(t),
    yAxis: {
      type: 'category',
      data: top.value.map((row) => target(row)),
      axisTick: { show: false },
      axisLine: { show: false },
      axisLabel: {
        color: t.ink,
        fontSize: 11,
        fontFamily: 'monospace',
        width: 130,
        overflow: 'truncate'
      }
    },
    series: [
      {
        type: 'bar',
        name: '去重点击',
        barMaxWidth: 16,
        data: top.value.map((row) => row.uniques),
        itemStyle: { color: t.accent, borderRadius: [0, 4, 4, 0] },
        label: {
          show: true,
          position: 'right',
          color: t.ink,
          fontSize: 11,
          formatter: (params: { value?: unknown }) => fmt(Number(params.value))
        }
      }
    ]
  }
})
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center gap-3">
      <div class="w-full sm:w-64">
        <KunInput
          v-model="query"
          placeholder="搜索商品 ID 或应用"
          size="sm"
          is-clearable
        />
      </div>
      <KunSelect
        v-model="kind"
        :options="STORE_LINK_KIND_FILTERS"
        size="sm"
        aria-label="链接类型"
        class-name="w-36"
      />
      <KunSelect
        v-model="sort"
        :options="STORE_LINK_SORTS"
        size="sm"
        aria-label="排序方式"
        class-name="w-40"
      />
      <span class="text-default-400 ml-auto text-xs">
        {{ fmt(filtered.length) }} / {{ fmt(rows.length) }} 条
      </span>
    </div>

    <KunCard
      v-if="filtered.length"
      content-class="justify-start items-stretch gap-0"
      class-name="p-6"
    >
      <h3 class="text-foreground mb-4 text-sm font-semibold">
        点击最多的 {{ Math.min(STORE_TOP_LINKS, filtered.length) }} 条
      </h3>
      <ChartFrame :option="option" :height="top.length * 34 + 24" />
    </KunCard>

    <KunCard
      v-if="paged.length"
      content-class="p-0"
      class-name="overflow-hidden"
    >
      <div class="overflow-x-auto">
        <table class="w-full min-w-[44rem] text-sm">
          <thead>
            <tr class="border-default-200 text-default-400 border-b text-left">
              <th class="px-4 py-2 font-medium">商品 / 活动</th>
              <th class="px-4 py-2 font-medium">应用</th>
              <th class="px-4 py-2 font-medium">类型</th>
              <th class="px-4 py-2 text-right font-medium">去重点击</th>
              <th class="w-32 px-4 py-2 font-medium">占比</th>
              <th class="px-4 py-2 text-right font-medium">总点击</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(row, i) in paged"
              :key="rowKey(row)"
              class="border-default-100 border-b"
              :class="i === paged.length - 1 && 'border-b-0'"
            >
              <td class="text-foreground px-4 py-2 font-mono">
                {{ target(row) }}
              </td>
              <td class="px-4 py-2">
                <NuxtLink
                  :to="`/dashboard/apps/${row.client_id}`"
                  class="text-default-500 hover:text-primary hover:underline"
                >
                  {{ row.app_name }}
                </NuxtLink>
              </td>
              <td class="px-4 py-2">
                <KunChip
                  :color="STORE_LINK_KIND_COLORS[row.kind]"
                  variant="flat"
                  size="xs"
                >
                  {{ STORE_LINK_KIND_LABELS[row.kind] }}
                </KunChip>
              </td>
              <td
                class="text-foreground px-4 py-2 text-right font-medium tabular-nums"
              >
                {{ fmt(row.uniques) }}
              </td>
              <td class="px-4 py-2">
                <div
                  class="bg-default-100 h-1.5 w-full overflow-hidden rounded-full"
                >
                  <div
                    class="bg-primary h-full rounded-full"
                    :style="{ width: share(row) }"
                  />
                </div>
              </td>
              <td class="text-default-400 px-4 py-2 text-right tabular-nums">
                {{ fmt(row.total) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </KunCard>

    <KunCard v-else content-class="p-10">
      <p class="text-default-400 text-center">
        {{
          rows.length
            ? '没有符合筛选条件的链接。'
            : '这段时间还没有链接被点开。'
        }}
      </p>
    </KunCard>

    <KunPagination
      v-if="totalPage > 1"
      v-model:current-page="page"
      :total-page="totalPage"
      class="justify-center"
    />
  </div>
</template>
