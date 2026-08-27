<script setup lang="ts">
import { STORE_LINK_KIND_COLORS, STORE_LINK_KIND_LABELS } from '~/constants/store'
import type { StoreUsageLink } from '~~/shared/types/store'

defineProps<{ rows: StoreUsageLink[] }>()

const fmt = (n: number) => n.toLocaleString()

const target = (row: StoreUsageLink) =>
  row.product_id ?? (row.campaign_id === null ? '—' : `#${row.campaign_id}`)
</script>

<template>
  <KunCard v-if="rows.length" content-class="p-0" class-name="overflow-hidden">
    <div class="overflow-x-auto">
      <table class="w-full min-w-[36rem] text-sm">
        <thead>
          <tr class="border-b border-default-200 text-left text-default-400">
            <th class="px-4 py-2 font-medium">应用</th>
            <th class="px-4 py-2 font-medium">类型</th>
            <th class="px-4 py-2 font-medium">商品 / 活动</th>
            <th class="px-4 py-2 text-right font-medium">去重点击</th>
            <th class="px-4 py-2 text-right font-medium">总点击</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(row, i) in rows"
            :key="`${row.client_id}-${row.kind}-${row.product_id ?? row.campaign_id}`"
            class="border-b border-default-100"
            :class="i === rows.length - 1 && 'border-b-0'"
          >
            <td class="px-4 py-2">
              <NuxtLink
                :to="`/apps/${row.client_id}`"
                class="font-medium text-foreground hover:text-primary hover:underline"
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
            <td class="px-4 py-2 font-mono text-default-500">
              {{ target(row) }}
            </td>
            <td class="px-4 py-2 text-right font-medium text-foreground">
              {{ fmt(row.uniques) }}
            </td>
            <td class="px-4 py-2 text-right text-default-400">
              {{ fmt(row.total) }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </KunCard>

  <KunCard v-else content-class="p-10">
    <p class="text-center text-default-400">这段时间还没有链接被点开。</p>
  </KunCard>
</template>
