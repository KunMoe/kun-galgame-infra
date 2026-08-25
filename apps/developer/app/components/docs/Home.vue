<script setup lang="ts">
import { API_BASE_URL } from '~/constants/dev'
import { DOCS_FACE_META } from '~/constants/docs'
import { ATTRIBUTION_NOTE } from '~~/shared/brand.mjs'

const { faces, findFace, faceOperationCount } = useDocs()

useSeoMeta({
  title: 'API 文档',
  description:
    'NextMoe 开放 API 参考：目录数据（只读，API 密钥）、游玩时长、编辑提案（用户令牌）与资讯（授权制）四个 API。鉴权、限流，以及每个端点的参数 / 响应 / curl 示例。'
})

const totalOperations = computed(() =>
  faces.reduce((n, f) => n + faceOperationCount(f), 0)
)

const countOf = (key: string) => {
  const face = findFace(key)
  return face ? faceOperationCount(face) : 0
}

const catalogOperations = computed(() => countOf('catalog'))
</script>

<template>
  <div class="space-y-10">
    <header>
      <p class="text-sm font-medium tracking-wide text-primary">API 参考</p>
      <h1 class="mt-2 text-3xl font-bold tracking-tight text-foreground">
        NextMoe 开放 API
      </h1>
      <p class="mt-3 max-w-2xl text-default-500">
        一个 base URL、{{ totalOperations }} 个公开端点（含 v1 与 v2）。主力是
        {{ catalogOperations }} 条只读的目录数据端点：同一部作品在 VNDB、Bangumi、DLsite、ErogameScape、Ci-en、Getchu
        六个源各有一个页面，我们把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上这个答案取自哪个源。
      </p>
      <p class="mt-3 max-w-2xl text-sm text-default-400">
        {{ ATTRIBUTION_NOTE }}
      </p>
    </header>

    <section class="grid gap-4 md:grid-cols-3">
      <div class="rounded-xl border border-default-200 bg-content1 p-4">
        <h2 class="flex items-center gap-2 text-sm font-semibold text-foreground">
          <KunIcon name="lucide:link" class="size-4 text-primary" />
          Base URL
        </h2>
        <div class="mt-3 flex items-center justify-between gap-2">
          <code class="min-w-0 flex-1 truncate font-mono text-sm text-foreground">
            {{ API_BASE_URL }}
          </code>
          <DocsCopyButton :text="API_BASE_URL" label="复制 base URL" />
        </div>
      </div>

      <div class="rounded-xl border border-default-200 bg-content1 p-4">
        <h2 class="flex items-center gap-2 text-sm font-semibold text-foreground">
          <KunIcon name="lucide:shield-check" class="size-4 text-primary" />
          两种凭据
        </h2>
        <p class="mt-3 text-sm leading-relaxed text-default-500">
          读目录用
          <strong class="text-foreground">API 密钥</strong>（<code
            class="rounded bg-default-100 px-1 py-0.5 font-mono text-xs text-foreground"
            >Authorization: Bearer nm_live_…</code
          >，机密，只放在服务端）；读写某个用户自己的东西——游玩时长、编辑提案——改用那个用户
          <strong class="text-foreground">授权后的访问令牌</strong>。目录规模统计
          <code class="font-mono text-xs text-foreground">/v1/catalog/stats</code>
          两者都不要，匿名即可调。
        </p>
      </div>

      <div class="rounded-xl border border-default-200 bg-content1 p-4">
        <h2 class="flex items-center gap-2 text-sm font-semibold text-foreground">
          <KunIcon name="lucide:gauge" class="size-4 text-primary" />
          限流
        </h2>
        <p class="mt-3 text-sm text-default-500">
          按 tier 分层（free 60 次/分 · 50,000 次/日）。超限返回
          <code class="font-mono text-xs text-foreground">429</code>
          并携带
          <code class="font-mono text-xs text-foreground">X-RateLimit-*</code>
          头。
        </p>
      </div>
    </section>

    <section>
      <h2 class="text-lg font-semibold text-foreground">四个 API</h2>
      <p class="mt-1 text-sm text-default-500">
        每个 API 各占一段路径前缀，凭据也按前缀区分——下面每张卡上都写着它收哪一种。
      </p>
      <div class="mt-4 grid gap-4 md:grid-cols-2">
        <NuxtLink
          v-for="face in faces"
          :key="face.key"
          :to="`/docs/${face.key}`"
          class="group rounded-xl border border-default-200 bg-content1 p-5 transition-colors hover:border-primary"
        >
          <div class="flex items-center justify-between">
            <div
              class="flex size-11 items-center justify-center rounded-lg bg-default-100 text-foreground"
            >
              <KunIcon :name="DOCS_FACE_META[face.key].icon" class="size-5" />
            </div>
            <div class="flex items-center gap-2">
              <span
                v-if="DOCS_FACE_META[face.key].badge"
                class="rounded-full bg-warning-50 px-2 py-1 text-xs font-medium text-warning-600"
              >
                {{ DOCS_FACE_META[face.key].badge }}
              </span>
              <span
                class="rounded-full bg-default-100 px-2.5 py-1 text-xs font-medium text-default-500"
              >
                {{ faceOperationCount(face) }} 端点
              </span>
            </div>
          </div>
          <div class="mt-4 flex flex-wrap items-center gap-2">
            <h3 class="text-base font-semibold text-foreground">
              {{ face.name }}
            </h3>
            <code class="font-mono text-xs text-default-400">
              {{ face.prefix }}
            </code>
          </div>
          <p class="mt-1 text-sm leading-relaxed text-default-500">
            {{ DOCS_FACE_META[face.key].tagline }}
          </p>
          <span
            class="mt-4 inline-flex items-center gap-1 text-sm font-medium text-primary"
          >
            查看端点
            <KunIcon
              name="lucide:arrow-right"
              class="size-4 transition-transform group-hover:translate-x-0.5"
            />
          </span>
        </NuxtLink>
      </div>
    </section>

    <section>
      <h2 class="text-lg font-semibold text-foreground">实战示例</h2>
      <p class="mt-1 text-sm text-default-500">
        用两个真实系列（SEQUEL × いろセカ）走通 搜索 → 详情 → 厂牌 → 外部 id 反查 全链。
      </p>
      <NuxtLink
        to="/docs/example"
        class="group mt-4 flex items-center gap-4 rounded-xl border border-default-200 bg-content1 p-5 transition-colors hover:border-primary"
      >
        <div
          class="flex size-11 shrink-0 items-center justify-center rounded-lg bg-default-100 text-foreground"
        >
          <KunIcon name="lucide:book-open" class="size-5" />
        </div>
        <div class="min-w-0 flex-1">
          <h3 class="text-base font-semibold text-foreground">金样双系列 Walkthrough</h3>
          <p class="mt-1 text-sm leading-relaxed text-default-500">
            每段响应节选都是线上真实数据；顺带演示 nsfw 参数与多源并列的读法。
          </p>
        </div>
        <KunIcon
          name="lucide:arrow-right"
          class="size-4 shrink-0 text-default-400 transition-transform group-hover:translate-x-0.5"
        />
      </NuxtLink>
    </section>

    <section>
      <h2 class="text-lg font-semibold text-foreground">给 AI 用</h2>
      <p class="mt-1 text-sm text-default-500">
        目录数据与资讯两个 API 同时以 MCP（Model Context Protocol）server 暴露，AI
        助手可直接调用；游玩时长与编辑提案按裁定不进 MCP。
      </p>
      <NuxtLink
        to="/docs/mcp"
        class="group mt-4 flex items-center gap-4 rounded-xl border border-default-200 bg-content1 p-5 transition-colors hover:border-primary"
      >
        <div
          class="flex size-11 shrink-0 items-center justify-center rounded-lg bg-default-100 text-foreground"
        >
          <KunIcon name="lucide:bot" class="size-5" />
        </div>
        <div class="min-w-0 flex-1">
          <h3 class="text-base font-semibold text-foreground">AI / MCP 接入</h3>
          <p class="mt-1 text-sm leading-relaxed text-default-500">
            纯透传适配层：一个端点、同一把密钥、一整套只读工具。含 Claude Code /
            Claude Desktop / 通用客户端配置示例。
          </p>
        </div>
        <KunIcon
          name="lucide:arrow-right"
          class="size-4 shrink-0 text-primary transition-transform group-hover:translate-x-0.5"
        />
      </NuxtLink>
    </section>
  </div>
</template>
