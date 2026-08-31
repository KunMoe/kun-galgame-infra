<script setup lang="ts">
definePageMeta({ layout: 'docs' })

const { problems } = await import('~/generated/problems')
const { problemsZh, problemDomainsZh } = await import('~/generated/problems-zh')

useSeoMeta({
  title: '错误码 · Problem types',
  description:
    'NextMoe API v2 的 RFC 9457 错误类型注册表：每个顶层 code 的 HTTP 状态与含义。'
})

type ProblemRow = (typeof problems)[number]

const grouped = computed(() => {
  const map = new Map<string, ProblemRow[]>()
  for (const p of problems) {
    const list = map.get(p.domain) ?? []
    list.push(p)
    map.set(p.domain, list)
  }
  return [...map.entries()]
})
</script>

<template>
  <div class="space-y-8">
    <header>
      <p class="text-primary text-sm font-medium">错误契约</p>
      <h1 class="text-foreground mt-1 text-2xl font-bold">
        错误码
        <span class="text-default-400 ml-2 font-mono text-base font-normal">
          Problem types
        </span>
      </h1>
      <p class="text-default-500 mt-3 max-w-2xl text-sm leading-relaxed">
        v2 的每个错误体都带一个
        <code class="font-mono text-xs">type</code>
        URI，它解析到这里的某一页。下面是中文说明；每页同时保留英文原文，
        <code class="font-mono text-xs">code</code>
        这类线上标识一律不翻译。字段级的
        <code class="font-mono text-xs">errors[].reason</code>
        是另一套词表，与这里的
        <code class="font-mono text-xs">code</code>
        互不重叠——两层怎么分支见
        <NuxtLink to="/docs/errors" class="text-primary hover:underline">
          错误处理
        </NuxtLink>
        。
      </p>
    </header>

    <section v-for="[domain, rows] in grouped" :key="domain" class="space-y-3">
      <h2
        class="text-foreground flex items-baseline gap-2 text-sm font-semibold"
      >
        {{ problemDomainsZh[domain] ?? domain }}
        <span class="text-default-400 font-mono text-xs font-normal">
          {{ domain }} · {{ rows.length }}
        </span>
      </h2>
      <ul
        class="divide-default-200 border-default-200 bg-content1 divide-y overflow-hidden rounded-xl border"
      >
        <li v-for="p in rows" :key="p.code">
          <NuxtLink
            :to="`/problems/${p.domain}/${p.kebab}`"
            class="hover:bg-default-50 flex items-center gap-3 px-4 py-3 transition-colors"
          >
            <span
              class="text-default-400 w-12 shrink-0 font-mono text-xs"
              aria-label="HTTP 状态"
            >
              {{ p.status }}
            </span>
            <span class="min-w-0 flex-1">
              <span class="text-foreground block truncate font-mono text-sm">
                {{ p.code }}
              </span>
              <span class="text-default-500 block truncate text-sm">
                {{ problemsZh[p.code]?.title ?? p.title }}
              </span>
            </span>
            <KunIcon
              name="lucide:chevron-right"
              class="text-default-300 size-4 shrink-0"
            />
          </NuxtLink>
        </li>
      </ul>
    </section>
  </div>
</template>
