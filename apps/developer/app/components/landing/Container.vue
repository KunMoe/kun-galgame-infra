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
  <div>
    <section
      class="border-default-200 bg-content1 relative isolate overflow-hidden border-b"
    >
      <div class="kun-hero-glow pointer-events-none absolute inset-0 -z-10" />

      <div
        class="mx-auto grid max-w-7xl items-end px-4 md:px-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,30rem)] lg:gap-12"
      >
        <div
          class="pt-14 pb-10 text-center lg:self-center lg:py-24 lg:text-left"
        >
          <div
            class="border-default-200 bg-background text-default-500 inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-medium"
          >
            <span class="bg-success size-2 rounded-full" />
            公开 API · 六源对齐 · 免费调用
          </div>

          <h1
            class="text-foreground mt-7 text-[2.75rem] leading-[1.06] font-bold tracking-tight sm:text-6xl lg:text-7xl xl:text-[5rem]"
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
        </div>

        <!--
          self-end with no bottom padding on purpose: the asset is cropped to
          her silhouette, so her feet land exactly on the section's bottom
          border and she stands on the page instead of floating in a box.
        -->
        <div class="flex justify-center self-end lg:justify-end">
          <img
            src="/koi.webp"
            alt="NextMoe 看板娘 恋（Koi）向你伸出手"
            width="706"
            height="1538"
            fetchpriority="high"
            class="kun-koi block h-[20rem] w-auto object-contain sm:h-[26rem] lg:h-[38rem] xl:h-[44rem]"
          />
        </div>
      </div>
    </section>

    <section v-if="metrics.length" class="border-default-200 border-b">
      <div
        class="mx-auto grid max-w-7xl grid-cols-2 gap-y-8 px-4 py-12 md:grid-cols-5 md:px-6"
      >
        <div
          v-for="metric in metrics"
          :key="metric.label"
          class="text-center last:col-span-2 md:text-left md:last:col-span-1"
        >
          <p
            class="text-foreground text-3xl font-bold tracking-tight tabular-nums md:text-4xl"
          >
            {{ metric.value }}
          </p>
          <p class="text-default-400 mt-1.5 text-xs">{{ metric.label }}</p>
        </div>
      </div>
    </section>

    <section class="mx-auto max-w-7xl px-4 py-16 md:px-6 md:py-20">
      <div
        class="grid gap-8 lg:grid-cols-[minmax(0,19rem)_minmax(0,1fr)] lg:items-center lg:gap-14"
      >
        <div>
          <h2 class="text-foreground text-2xl font-bold tracking-tight">
            一条命令就能验完
          </h2>
          <p class="text-default-500 mt-3 text-sm leading-relaxed">
            登录后自助铸一把
            <code class="text-default-600 font-mono text-xs">nmk_live_</code>
            密钥，不需要申请，也没有付费档位。
          </p>
          <NuxtLink
            to="/docs/quickstart"
            class="text-primary mt-4 inline-flex items-center gap-1 text-sm font-medium hover:underline"
          >
            快速开始
            <KunIcon name="lucide:arrow-right" class="size-4" />
          </NuxtLink>
        </div>

        <div
          class="border-default-200 bg-content1 overflow-hidden rounded-xl border"
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
            class="text-default-600 overflow-x-auto px-4 py-4 text-left font-mono text-xs leading-relaxed sm:text-sm"
          ><code>{{ curlSample }}</code></pre>
        </div>
      </div>

      <p v-if="metrics.length" class="text-default-400 mt-10 text-xs">
        上方数字实时取自
        <code class="text-default-500 font-mono">/v2/catalog/stats</code>
        <template v-if="galgameCount"
          >，其中 Galgame {{ galgameCount }} 部</template
        >
        。这个端点本身不需要任何凭据。
      </p>
    </section>
  </div>
</template>
