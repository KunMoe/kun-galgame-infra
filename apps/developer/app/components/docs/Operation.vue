<script setup lang="ts">
import { cn } from '@kungal/ui-core'

const route = useRoute()
const { findOperation } = useDocs()

const resolved = findOperation(
  route.params.face as string,
  route.params.operationId as string
)
if (!resolved) {
  throw createError({ statusCode: 404, statusMessage: '未找到该端点', fatal: true })
}
const { face, operation } = resolved

const fullUrl = `${face.baseUrl}${operation.path}`
const auth = operation.auth ?? face.auth

useSeoMeta({
  title: `${operation.id} · ${face.name}`,
  description: operation.summary
})

const openResp = reactive<Record<string, boolean>>({})
operation.responses.forEach((r) => {
  openResp[r.status] = r.status === '200'
})
const toggleResp = (status: string) => {
  openResp[status] = !openResp[status]
}

const statusMeta = (status: string): { label: string; class: string } => {
  if (status === 'default') {
    return { label: '默认（错误）', class: 'bg-danger-50 text-danger-600' }
  }
  const code = Number(status)
  if (code >= 200 && code < 300) {
    return { label: `${status} OK`, class: 'bg-success-50 text-success-600' }
  }
  if (code >= 400 && code < 500) {
    return { label: status, class: 'bg-warning-50 text-warning-600' }
  }
  return { label: status, class: 'bg-danger-50 text-danger-600' }
}
</script>

<template>
  <article class="space-y-8">
    <nav class="flex flex-wrap items-center gap-1.5 text-sm text-default-400">
      <NuxtLink to="/docs" class="transition-colors hover:text-foreground">
        文档
      </NuxtLink>
      <KunIcon name="lucide:chevron-right" class="size-3.5" />
      <NuxtLink
        :to="`/docs/${face.key}`"
        class="transition-colors hover:text-foreground"
      >
        {{ face.label }}
      </NuxtLink>
      <KunIcon name="lucide:chevron-right" class="size-3.5" />
      <code class="font-mono text-default-500">{{ operation.id }}</code>
    </nav>

    <header class="space-y-3">
      <h1 class="text-2xl font-bold tracking-tight text-foreground">
        {{ operation.id }}
      </h1>
      <p class="text-default-500">{{ operation.summary }}</p>

      <div
        class="flex items-center gap-3 overflow-x-auto rounded-xl border border-default-200 bg-content1 px-4 py-3"
      >
        <DocsMethodBadge :method="operation.method" size="md" />
        <code class="flex-1 font-mono text-sm whitespace-nowrap text-foreground">
          {{ operation.path }}
        </code>
        <DocsCopyButton :text="fullUrl" label="复制请求地址" />
      </div>

      <div
        class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-default-400"
      >
        <span v-if="operation.scope" class="flex items-center gap-1.5">
          <KunIcon name="lucide:shield-check" class="size-3.5" />
          scope
          <code class="rounded bg-default-100 px-1 py-0.5 font-mono text-default-600">
            {{ operation.scope }}
          </code>
        </span>
        <span class="flex items-center gap-1.5">
          <KunIcon
            :name="auth.kind === 'none' ? 'lucide:lock-open' : 'lucide:key-round'"
            class="size-3.5"
          />
          <code v-if="auth.curl" class="font-mono">{{ auth.display }}</code>
          <span v-else class="font-medium text-success-600">
            {{ auth.display }}
          </span>
          <span>（{{ auth.note }}）</span>
        </span>
      </div>
    </header>

    <section v-if="operation.params.length" class="space-y-3">
      <h2 class="text-lg font-semibold text-foreground">参数</h2>
      <DocsParamTable :params="operation.params" />
    </section>

    <section v-if="operation.requestBody" class="space-y-3">
      <h2 class="text-lg font-semibold text-foreground">请求体</h2>
      <p class="text-xs text-default-400">
        <code class="font-mono">application/json</code>
      </p>
      <div class="rounded-xl border border-default-200 bg-content1 p-4">
        <DocsSchemaTree :node="operation.requestBody" />
      </div>
    </section>

    <section class="space-y-3">
      <h2 class="text-lg font-semibold text-foreground">响应</h2>
      <div
        v-for="res in operation.responses"
        :key="res.status"
        class="overflow-hidden rounded-xl border border-default-200"
      >
        <button
          type="button"
          class="flex w-full items-center gap-3 bg-content1 px-4 py-3 text-left transition-colors hover:bg-default-50"
          :aria-expanded="openResp[res.status]"
          @click="toggleResp(res.status)"
        >
          <span
            :class="
              cn(
                'rounded px-2 py-0.5 font-mono text-xs font-bold',
                statusMeta(res.status).class
              )
            "
          >
            {{ statusMeta(res.status).label }}
          </span>
          <span class="min-w-0 flex-1 truncate text-sm text-default-500">
            {{ res.description }}
          </span>
          <KunIcon
            v-if="res.schema"
            :name="
              openResp[res.status] ? 'lucide:chevron-up' : 'lucide:chevron-down'
            "
            class="size-4 shrink-0 text-default-400"
          />
        </button>
        <div
          v-if="res.schema && openResp[res.status]"
          class="border-t border-default-200 p-4"
        >
          <DocsSchemaTree :node="res.schema" />
        </div>
      </div>
    </section>

    <section class="space-y-3">
      <h2 class="text-lg font-semibold text-foreground">请求示例</h2>
      <DocsCurlBlock :code="operation.curl" />
    </section>
  </article>
</template>
