<script setup lang="ts">
import { STORE_SCOPE, STORE_USAGE_DAY_OPTIONS } from '~/constants/store'
import type { StoreUsageSummary } from '~~/shared/types/store'

useSeoMeta({ title: '分销链接', robots: 'noindex' })

const days = ref(30)

const { data } = await useApiFetch<StoreUsageSummary>(
  () => `/dev/store/usage?days=${days.value}`
)

const summary = computed(() => data.value)
const daily = computed(() => summary.value?.daily ?? [])
const byApp = computed(() => summary.value?.by_app ?? [])
const byLink = computed(() => summary.value?.by_link ?? [])
const linkCount = computed(() => summary.value?.link_count ?? 0)
const total = computed(() => summary.value?.total ?? 0)
const uniques = computed(() => summary.value?.uniques ?? 0)

const onboarded = computed(() => linkCount.value > 0)

const fmt = (n: number) => n.toLocaleString()

const dedupRate = computed(() =>
  total.value > 0 ? ((uniques.value / total.value) * 100).toFixed(0) : '—'
)

const tiles = computed(() => [
  { label: '去重点击', value: fmt(uniques.value) },
  { label: '总点击', value: fmt(total.value) },
  { label: '去重占比', value: dedupRate.value === '—' ? '—' : `${dedupRate.value}%` },
  { label: '已铸链接', value: fmt(linkCount.value) }
])
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-foreground">分销链接</h1>
        <p class="mt-1 text-sm text-default-500">
          你名下应用的 DLsite 分销短链点击量,按 JST 日统计。
        </p>
      </div>
      <div v-if="onboarded" class="flex rounded-lg border border-default-200 p-0.5">
        <button
          v-for="opt in STORE_USAGE_DAY_OPTIONS"
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

    <KunCard v-if="!onboarded" content-class="items-start gap-4" class-name="p-8">
      <div class="flex items-center gap-3">
        <KunIcon name="lucide:shopping-bag" class="size-5 text-primary" />
        <h2 class="text-lg font-semibold text-foreground">还没有接入分销链接</h2>
      </div>
      <p class="max-w-2xl text-sm leading-relaxed text-default-500">
        分销链接 API 会给你的站点单独签发一条 DLsite 短链:同一个商品,每个站拿到的链接都不一样,
        点击才归得到你这里。活动期还会附带优惠券领取链接。
      </p>
      <p class="max-w-2xl text-sm leading-relaxed text-default-500">
        先在控制台铸密钥时勾选
        <code class="rounded bg-default-100 px-1.5 py-0.5 font-mono text-xs">
          {{ STORE_SCOPE }}
        </code>
        ,然后调一次
        <code class="rounded bg-default-100 px-1.5 py-0.5 font-mono text-xs">
          GET /v2/store/purchase-links/{product_id}
        </code>
        就会铸出你的第一条链接,数据随后出现在这里。
      </p>
      <div class="flex flex-wrap gap-3">
        <KunButton color="primary" size="md" @click="navigateTo('/dashboard')">
          去控制台铸密钥
          <KunIcon name="lucide:arrow-right" class="ml-1 size-4" />
        </KunButton>
        <KunButton variant="flat" size="md" @click="navigateTo('/docs/v2')">
          <KunIcon name="lucide:book-open" class="mr-1 size-4" />
          阅读接入文档
        </KunButton>
      </div>
    </KunCard>

    <template v-else>
      <div class="grid grid-cols-2 gap-4 md:grid-cols-4">
        <div
          v-for="tile in tiles"
          :key="tile.label"
          class="rounded-xl border border-default-200 bg-content1 px-5 py-4"
        >
          <p class="text-xs text-default-400">{{ tile.label }}</p>
          <p class="mt-1 text-2xl font-bold text-foreground">{{ tile.value }}</p>
        </div>
      </div>

      <KunCard content-class="justify-start gap-0 items-stretch" class-name="p-6">
        <div class="mb-5 flex flex-wrap items-center justify-between gap-2">
          <h2 class="text-base font-semibold text-foreground">每日点击</h2>
          <span class="text-xs text-default-400">
            深色为去重点击 · 最近 {{ days }} 天(JST)
          </span>
        </div>
        <StoreChart :days="daily" />
      </KunCard>

      <div>
        <h2 class="mb-3 text-lg font-semibold text-foreground">按应用</h2>
        <KunCard content-class="p-0" class-name="overflow-hidden">
          <div class="overflow-x-auto">
            <table class="w-full min-w-[32rem] text-sm">
              <thead>
                <tr class="border-b border-default-200 text-left text-default-400">
                  <th class="px-4 py-2 font-medium">应用</th>
                  <th class="px-4 py-2 text-right font-medium">链接数</th>
                  <th class="px-4 py-2 text-right font-medium">去重点击</th>
                  <th class="px-4 py-2 text-right font-medium">总点击</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(row, i) in byApp"
                  :key="row.client_id"
                  class="border-b border-default-100"
                  :class="i === byApp.length - 1 && 'border-b-0'"
                >
                  <td class="px-4 py-2">
                    <NuxtLink
                      :to="`/apps/${row.client_id}`"
                      class="font-medium text-foreground hover:text-primary hover:underline"
                    >
                      {{ row.name }}
                    </NuxtLink>
                  </td>
                  <td class="px-4 py-2 text-right text-default-500">
                    {{ fmt(row.links) }}
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
      </div>

      <div>
        <h2 class="mb-3 text-lg font-semibold text-foreground">按链接</h2>
        <StoreTable :rows="byLink" />
      </div>

      <p class="text-xs leading-relaxed text-default-400">
        去重口径:同一条短链、同一个 JST 日、同一个指纹(访问 IP 与 User-Agent 的
        SHA-256)只算一次——这是我们对 DLsite 的承诺。计数每小时从跳转器同步一次,当天数字始终是不完整的。
      </p>
    </template>
  </div>
</template>
