<script setup lang="ts">
import { API_BASE_URL } from '~/constants/dev'
import type { CatalogStats } from '~~/shared/types/stats'

const auth = useAuth()
const { open: openLogin } = useLoginModal()

useSeoMeta({
  title: '开发者平台',
  description:
    'NextMoe 开发者平台 — ACGN 数据，以此为准。同一部作品在 VNDB、Bangumi、DLsite、ErogameScape、Ci-en、Getchu 六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。数据完全免费调用。',
  ogTitle: 'NextMoe 开发者平台',
  ogDescription: 'ACGN 数据，以此为准 — one canon, every source reconciled.',
  ogImage: '/android-chrome-512x512.png'
})

// The portal holds no catalog key of its own, and api.nextmoe.dev's CORS
// allowlist does not carry developer.nextmoe.dev — a browser fetch straight at
// the absolute URL is blocked on every client-side navigation to this page.
// Same-origin relay instead, the pattern /explore already uses.
interface V2Stats {
  object?: string
  works?: number
  companies?: number
  characters?: number
  credit_names?: number
  persons?: number
}

const { data: stats } = await useFetch<CatalogStats | V2Stats>(
  '/relay/v2/catalog/stats',
  { key: 'landing-catalog-stats', timeout: 5000 }
)

const counts = computed(() => {
  const s = stats.value as (CatalogStats & V2Stats) | null
  if (!s) return null
  if (
    'works' in s &&
    typeof s.works === 'object' &&
    s.works &&
    'total' in s.works
  ) {
    return s as CatalogStats
  }
  if (typeof s.works === 'number') {
    return {
      works: { total: s.works, by_medium: [] },
      entities: {
        labels: s.companies ?? 0,
        characters: s.characters ?? 0,
        credit_names: s.credit_names ?? 0,
        persons: s.persons ?? 0
      }
    } satisfies CatalogStats
  }
  return null
})

const formatCount = (n: number): string => {
  if (n < 10000) return String(n)
  const wan = n / 10000
  return `${wan >= 1000 ? Math.round(wan) : Number(wan.toFixed(1))} 万`
}

const metrics = computed(() => {
  const c = counts.value
  if (!c) return []
  return [
    { value: formatCount(c.works.total), label: '作品' },
    { value: formatCount(c.entities.characters), label: '角色' },
    { value: formatCount(c.entities.labels), label: '厂牌 · 社团' },
    { value: formatCount(c.entities.credit_names), label: '制作人员名义' },
    { value: formatCount(c.entities.persons), label: '人物' }
  ]
})

const galgameCount = computed(() => {
  const row = counts.value?.works.by_medium.find((m) => m.medium === 'galgame')
  return row ? formatCount(row.count) : ''
})

const curlSample = `curl https://api.nextmoe.dev/v2/catalog/works/1 \\
  -H "Authorization: Bearer nmk_live_…"`
</script>

<template>
  <div class="space-y-16 md:space-y-24">
    <section
      class="grid items-center gap-10 pt-2 md:pt-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,26rem)] lg:gap-16"
    >
      <div class="text-center lg:text-left">
        <div
          class="border-default-200 bg-content1 text-default-500 inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-medium"
        >
          <span class="bg-success size-2 rounded-full" />
          公开 API · 六源对齐 · 免费调用
        </div>

        <h1
          class="text-foreground mt-7 text-5xl leading-[1.05] font-bold tracking-tight md:text-6xl lg:text-7xl"
        >
          ACGN 数据，<br />
          以此为准
        </h1>
        <p
          class="text-primary mt-5 font-mono text-sm tracking-wide lg:text-base"
        >
          One canon. Every source reconciled.
        </p>
        <p
          class="text-default-500 mx-auto mt-6 max-w-xl text-base leading-relaxed lg:mx-0 lg:text-lg"
        >
          同一部作品在六个上游各有一个页面，写的还常常不一样。NextMoe
          把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上这个答案取自哪个源。
        </p>

        <div
          class="mt-9 flex flex-wrap items-center justify-center gap-3 lg:justify-start"
        >
          <KunButton
            v-if="auth.isLoggedIn.value"
            color="primary"
            size="lg"
            @click="navigateTo('/dashboard')"
          >
            进入控制台
            <KunIcon name="lucide:arrow-right" class="ml-1 size-4" />
          </KunButton>
          <KunButton v-else color="primary" size="lg" @click="openLogin()">
            登录开始
            <KunIcon name="lucide:arrow-right" class="ml-1 size-4" />
          </KunButton>
          <KunButton variant="flat" size="lg" @click="navigateTo('/docs')">
            <KunIcon name="lucide:book-open" class="mr-1 size-4" />
            查看 API 文档
          </KunButton>
        </div>

        <div
          class="border-default-200 bg-content1 mx-auto mt-9 max-w-xl overflow-hidden rounded-xl border lg:mx-0"
        >
          <div
            class="border-default-200 flex items-center gap-2 border-b px-4 py-2.5"
          >
            <span class="bg-danger/40 size-2.5 rounded-full" />
            <span class="bg-warning/40 size-2.5 rounded-full" />
            <span class="bg-success/40 size-2.5 rounded-full" />
            <span class="text-default-400 ml-1.5 font-mono text-xs">
              {{ API_BASE_URL }}
            </span>
            <DocsCopyButton
              :text="curlSample"
              label="复制 curl 示例"
              class="ml-auto"
            />
          </div>
          <pre
            class="text-default-600 overflow-x-auto px-4 py-3.5 text-left font-mono text-xs leading-relaxed"
          ><code>{{ curlSample }}</code></pre>
        </div>
      </div>

      <div class="flex justify-center lg:justify-end">
        <div class="relative flex flex-col items-center">
          <div
            class="bg-primary-50 pointer-events-none absolute bottom-10 aspect-square w-[118%] rounded-full"
          />
          <img
            src="/koi.webp"
            alt="NextMoe 看板娘 恋（Koi）向你伸出手"
            width="941"
            height="1672"
            fetchpriority="high"
            class="relative h-[22rem] w-auto object-contain md:h-[26rem] lg:h-[32rem]"
          />
          <p class="text-default-300 relative mt-1 self-end font-mono text-xs">
            恋 / Koi
          </p>
        </div>
      </div>
    </section>

    <section v-if="metrics.length">
      <div
        class="border-default-200 bg-default-200 grid grid-cols-2 gap-px border-y md:grid-cols-5"
      >
        <div
          v-for="metric in metrics"
          :key="metric.label"
          class="bg-background px-4 py-8 text-center last:col-span-2 md:last:col-span-1"
        >
          <p
            class="text-foreground text-3xl font-bold tracking-tight md:text-4xl"
          >
            {{ metric.value }}
          </p>
          <p class="text-default-400 mt-1.5 text-xs">{{ metric.label }}</p>
        </div>
      </div>
      <p class="text-default-400 mt-4 text-center text-xs">
        实时取自
        <code class="text-default-500 font-mono">/v2/catalog/stats</code>
        <template v-if="galgameCount">
          ，其中 Galgame {{ galgameCount }} 部
        </template>
        。这个端点本身不需要任何凭据。
      </p>
    </section>
  </div>
</template>
