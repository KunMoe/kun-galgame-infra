<script setup lang="ts">
import { API_BASE_URL } from '~/constants/dev'
import { ATTRIBUTION_NOTE, SOURCES } from '~~/shared/brand.mjs'
import type { CatalogStats } from '~~/shared/types/stats'

const auth = useAuth()
const { open: openLogin } = useLoginModal()

useSeoMeta({
  title: '开发者平台',
  description:
    'NextMoe 开发者平台 — ACGN 数据，以此为准。同一部作品在 VNDB、Bangumi、DLsite、ErogameScape、Ci-en、Getchu 六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。数据完全免费调用。',
  ogTitle: 'NextMoe 开发者平台',
  ogDescription: 'ACGN 数据，以此为准 — one canon, every source reconciled.'
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
  if ('works' in s && typeof s.works === 'object' && s.works && 'total' in s.works) {
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

const sources = SOURCES.map(([name, body]) => ({ name, body }))

const quickstart = [
  {
    icon: 'lucide:log-in',
    title: '登录生态账号',
    body: '用你的 NextMoe（鲲 Galgame）账号登录，不必另外注册开发者身份。'
  },
  {
    icon: 'lucide:layout-dashboard',
    title: '创建应用',
    body: '在控制台创建一个应用（每个账号最多 5 个），即刻拿到独立的配额与用量视图。'
  },
  {
    icon: 'lucide:key-round',
    title: '领取密钥',
    body: '生成密钥并妥善保存（只显示一次）。带上它请求只读端点；要读写用户自己的游玩记录或提交编辑提案，则改用那个用户授权后的访问令牌。'
  }
]

// One prefix = one credential, and the four of them are the whole public
// surface. Every card lands on the same reference page: v2 is one document.
const faces = [
  {
    key: 'catalog',
    path: '/v2/catalog',
    icon: 'lucide:network',
    name: '目录数据（只读）',
    tagline:
      '作品、角色、厂牌、制作人员的统一条目库。同一部作品在六个源各有一个页面，我们把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上这个答案取自哪个源。凭据是应用密钥。'
  },
  {
    key: 'news',
    path: '/v2/news',
    icon: 'lucide:newspaper',
    name: '资讯（无需凭据）',
    tagline:
      '合作媒体的 Galgame 资讯索引：标题、摘要、题图与回源链接，正文不下发。这个面不要任何凭据，直接调。'
  },
  {
    key: 'me',
    path: '/v2/me',
    icon: 'lucide:user-round',
    name: '用户面（用户令牌）',
    tagline:
      '代表某个用户读写他自己的东西：游玩时长、封面投票、认领、编辑提案、资讯投稿。凭据是那个用户授权后的访问令牌，不是应用密钥。'
  },
  {
    key: 'moderation',
    path: '/v2/moderation',
    icon: 'lucide:shield-check',
    name: '审核面（用户令牌 + 权限）',
    tagline:
      '一方站点的审核台：认领与提案队列、裁决、回滚、快照。用户令牌加审核权限，且按站点隔离。'
  }
] as const

const curlSample = `curl https://api.nextmoe.dev/v2/catalog/works/1 \\
  -H "Authorization: Bearer nmk_live_…"`
</script>

<template>
  <div class="space-y-20">
    <section
      class="grid items-center gap-10 pt-4 md:pt-10 lg:grid-cols-2 lg:gap-14"
    >
      <div class="text-center lg:text-left">
        <div
          class="inline-flex items-center gap-2 rounded-full border border-default-200 bg-content1 px-3 py-1 text-xs font-medium text-default-500"
        >
          <span class="size-2 rounded-full bg-success" />
          公开 API · 六源对齐 · 免费调用
        </div>

        <h1
          class="mt-6 text-4xl font-bold tracking-tight text-foreground md:text-5xl md:leading-[1.1] lg:text-6xl"
        >
          ACGN 数据，<br class="hidden sm:inline" />
          以此为准
        </h1>
        <p
          class="mt-4 text-sm font-medium tracking-wide text-primary lg:text-base"
        >
          One canon. Every source reconciled.
        </p>
        <p
          class="mx-auto mt-5 max-w-xl text-lg leading-relaxed text-default-500 lg:mx-0"
        >
          同一部作品，在 VNDB、Bangumi、DLsite、ErogameScape、Ci-en、Getchu
          六个源各有一个页面，写的还常常不一样。NextMoe
          把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上这个答案取自哪个源。
        </p>

        <div
          class="mt-8 flex flex-wrap items-center justify-center gap-3 lg:justify-start"
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
          class="mt-6 inline-flex items-center gap-2 rounded-lg border border-default-200 bg-content1 px-3 py-1.5"
        >
          <span class="text-xs text-default-400">Base URL</span>
          <span class="font-mono text-sm text-foreground">{{
            API_BASE_URL
          }}</span>
          <DocsCopyButton :text="API_BASE_URL" label="复制 Base URL" />
        </div>
      </div>

      <div
        class="overflow-hidden rounded-2xl border border-default-200 bg-content1 shadow-sm"
      >
        <div
          class="flex items-center gap-2 border-b border-default-200 px-4 py-3"
        >
          <span class="size-3 rounded-full bg-danger/50" />
          <span class="size-3 rounded-full bg-warning/50" />
          <span class="size-3 rounded-full bg-success/50" />
          <span class="ml-2 font-mono text-xs text-default-400">
            api.nextmoe.dev
          </span>
          <DocsCopyButton
            :text="curlSample"
            label="复制 curl 示例"
            class="ml-auto"
          />
        </div>
        <div class="space-y-4 px-4 py-4 font-mono text-xs leading-relaxed">
          <pre
            class="overflow-x-auto text-default-600"
          ><code>{{ curlSample }}</code></pre>
          <div class="space-y-1 border-t border-default-200 pt-4">
            <p class="font-semibold text-success">200 OK</p>
            <p class="text-default-400">
              cache-control:
              <span class="text-default-600">
                public, max-age=60, s-maxage=300
              </span>
            </p>
            <p class="text-default-400">
              etag:
              <span class="text-default-600">"w1.7c1f9a2b"</span>
            </p>
            <p class="text-default-400">
              ratelimit:
              <span class="text-default-600">
                limit=100, remaining=96, reset=53
              </span>
            </p>
          </div>
        </div>
      </div>
    </section>

    <section v-if="metrics.length">
      <div
        class="grid grid-cols-2 gap-px overflow-hidden rounded-2xl border border-default-200 bg-default-200 md:grid-cols-5"
      >
        <div
          v-for="metric in metrics"
          :key="metric.label"
          class="bg-background px-4 py-7 text-center"
        >
          <p class="text-2xl font-bold text-foreground md:text-3xl">
            {{ metric.value }}
          </p>
          <p class="mt-1 text-xs text-default-500">{{ metric.label }}</p>
        </div>
      </div>
      <p class="mt-3 text-center text-xs text-default-400">
        实时取自
        <code class="font-mono text-default-500">/v2/catalog/stats</code>
        <template v-if="galgameCount">
          ，其中 Galgame {{ galgameCount }} 部
        </template>
        。这个端点本身不需要任何凭据。
      </p>
    </section>

    <section>
      <div class="mb-8 text-center">
        <h2 class="text-2xl font-bold text-foreground md:text-3xl">
          数据从哪来
        </h2>
        <p class="mt-2 text-default-500">
          六个上游站点，加上未萌生态自己的一手数据与用户编辑。
        </p>
      </div>
      <div class="grid gap-4 md:grid-cols-3">
        <div
          v-for="source in sources"
          :key="source.name"
          class="rounded-xl border border-default-200 bg-content1 px-5 py-4"
        >
          <p class="text-base font-semibold text-foreground">
            {{ source.name }}
          </p>
          <p class="mt-1 text-sm text-default-500">{{ source.body }}</p>
        </div>
      </div>
      <div
        class="mt-4 grid gap-4 rounded-2xl border border-default-200 bg-content1 p-6 md:grid-cols-2"
      >
        <div>
          <h3
            class="flex items-center gap-2 text-base font-semibold text-foreground"
          >
            <KunIcon name="lucide:users" class="size-4 text-primary" />
            还有一手数据
          </h3>
          <p class="mt-2 text-sm leading-relaxed text-default-500">
            未萌（NextMoe）生态站点——鲲 Galgame
            论坛等——自己产出的条目、译名与整理，以及用户经编辑提案提交的修改，同样进入这一份记录。上游没有的东西，这里可能有。
          </p>
        </div>
        <div>
          <h3
            class="flex items-center gap-2 text-base font-semibold text-foreground"
          >
            <KunIcon name="lucide:heart-handshake" class="size-4 text-primary" />
            完全免费
          </h3>
          <p class="mt-2 text-sm leading-relaxed text-default-500">
            调用免费，编辑也免费：登录即可提交编辑提案，把你知道的补进来。没有付费档位，没有按量计费——只有一层防滥用的限流。
          </p>
        </div>
      </div>

      <div
        class="mt-4 rounded-2xl border border-default-200 bg-content1 px-6 py-5"
      >
        <h3
          class="flex items-center gap-2 text-base font-semibold text-foreground"
        >
          <KunIcon name="lucide:quote" class="size-4 text-primary" />
          品牌与署名
        </h3>
        <p class="mt-2 text-sm leading-relaxed text-default-500">
          {{ ATTRIBUTION_NOTE }}
        </p>
      </div>
    </section>

    <section>
      <div class="mb-8 text-center">
        <h2 class="text-2xl font-bold text-foreground md:text-3xl">三步开始</h2>
        <p class="mt-2 text-default-500">从登录到第一个成功请求，只需几分钟。</p>
      </div>
      <div class="grid gap-4 md:grid-cols-3">
        <KunCard
          v-for="(step, i) in quickstart"
          :key="step.title"
          :is-hoverable="false"
          content-class="justify-start gap-0 items-start"
          class-name="p-6 h-full"
        >
          <div class="flex items-center gap-3">
            <div
              class="flex size-10 items-center justify-center rounded-lg bg-primary-50 text-primary"
            >
              <KunIcon :name="step.icon" class="size-5" />
            </div>
            <span class="text-sm font-bold text-default-300">
              0{{ i + 1 }}
            </span>
          </div>
          <h3 class="mt-4 text-base font-semibold text-foreground">
            {{ step.title }}
          </h3>
          <p class="mt-1 text-sm leading-relaxed text-default-500">
            {{ step.body }}
          </p>
        </KunCard>
      </div>
    </section>

    <section>
      <div class="mb-8 text-center">
        <h2 class="text-2xl font-bold text-foreground md:text-3xl">
          四个面，四种凭据
        </h2>
        <p class="mt-2 text-default-500">
          一条前缀收一种凭据。v2 已正式公开，此后只做加法——删字段、改名字都会被 CI
          的破坏性变更门挡下。
        </p>
      </div>
      <div class="grid gap-4 md:grid-cols-2">
        <NuxtLink
          v-for="face in faces"
          :key="face.key"
          to="/docs/v2"
          class="group"
        >
          <KunCard
            :is-hoverable="true"
            content-class="justify-start gap-0 items-start"
            class-name="p-6 h-full"
          >
            <div class="flex w-full items-center justify-between">
              <div
                class="flex size-11 items-center justify-center rounded-lg bg-default-100 text-foreground"
              >
                <KunIcon :name="face.icon" class="size-5" />
              </div>
              <KunIcon
                name="lucide:arrow-up-right"
                class="size-4 text-default-300 transition-colors group-hover:text-primary"
              />
            </div>
            <div class="mt-4 flex flex-wrap items-center gap-2">
              <h3 class="text-base font-semibold text-foreground">
                {{ face.name }}
              </h3>
              <code class="font-mono text-xs text-default-400">{{
                face.path
              }}</code>
            </div>
            <p class="mt-1 text-sm leading-relaxed text-default-500">
              {{ face.tagline }}
            </p>
          </KunCard>
        </NuxtLink>
      </div>
    </section>

    <section
      class="rounded-2xl border border-default-200 bg-content1 px-6 py-12 text-center md:px-10"
    >
      <h2 class="text-2xl font-bold text-foreground md:text-3xl">
        几分钟内发出第一个请求
      </h2>
      <p class="mx-auto mt-3 max-w-xl text-default-500">
        登录生态账号，创建应用，领取密钥 —— 然后带上它请求任意公开端点。
      </p>
      <div class="mt-7 flex flex-wrap items-center justify-center gap-3">
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
          查看 API 文档
        </KunButton>
      </div>
    </section>
  </div>
</template>
