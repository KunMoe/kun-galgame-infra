<script setup lang="ts">
definePageMeta({ layout: 'docs' })

const route = useRoute()
const domain = computed(() => String(route.params.domain || ''))
const kebab = computed(() => String(route.params.code || ''))

const { problems } = await import('~/generated/problems')
const { problemsZh, problemDomainsZh } = await import('~/generated/problems-zh')

const entry = computed(() =>
  problems.find((p) => p.domain === domain.value && p.kebab === kebab.value)
)

if (!entry.value) {
  throw createError({
    statusCode: 404,
    statusMessage: 'Unknown problem type',
    fatal: true
  })
}

const zh = computed(() => problemsZh[entry.value!.code])

useSeoMeta({
  title: () => `${entry.value!.code} · 错误码`,
  description: () => zh.value?.description ?? entry.value!.description
})
</script>

<template>
  <div v-if="entry" class="space-y-6">
    <nav class="text-default-400 flex items-center gap-1.5 text-sm">
      <NuxtLink to="/docs" class="hover:text-foreground">文档</NuxtLink>
      <KunIcon name="lucide:chevron-right" class="size-3.5" />
      <NuxtLink to="/problems" class="hover:text-foreground">错误码</NuxtLink>
      <KunIcon name="lucide:chevron-right" class="size-3.5" />
      <span class="text-default-500">{{ entry.code }}</span>
    </nav>

    <header>
      <p class="text-default-400 font-mono text-xs break-all">
        {{ entry.type }}
      </p>
      <h1 class="text-foreground mt-2 text-2xl font-bold">
        {{ zh?.title ?? entry.title }}
      </h1>
      <div class="mt-3 flex flex-wrap items-center gap-2 text-xs">
        <code
          class="bg-default-100 text-foreground rounded-md px-2 py-1 font-mono"
        >
          {{ entry.code }}
        </code>
        <span
          class="bg-default-100 text-default-500 rounded-md px-2 py-1 font-mono"
        >
          HTTP {{ entry.status }}
        </span>
        <span class="bg-default-100 text-default-500 rounded-md px-2 py-1">
          {{ problemDomainsZh[entry.domain] ?? entry.domain }}
        </span>
      </div>
    </header>

    <p class="text-default-500 max-w-2xl text-sm leading-relaxed">
      {{ zh?.description ?? entry.description }}
    </p>

    <!--
      The English original stays on the page, not just in the twin: this URL is
      what every error body's `type` points at, so an agent that follows one
      lands here rather than on /problems.md, and `code`/`title`/`description`
      are the wire contract in the language it was published in.
    -->
    <section class="border-default-200 bg-content1 rounded-xl border p-5">
      <h2
        class="text-default-400 text-xs font-semibold tracking-wide uppercase"
      >
        Canonical (English)
      </h2>
      <p class="text-foreground mt-2 text-sm font-medium">{{ entry.title }}</p>
      <p class="text-default-500 mt-1 text-sm leading-relaxed">
        {{ entry.description }}
      </p>
    </section>

    <p class="text-default-400 text-sm">
      错误体的字段、两层注册表与分支顺序见
      <NuxtLink to="/docs/errors" class="text-primary hover:underline">
        错误处理
      </NuxtLink>
      ；
      <NuxtLink to="/problems" class="text-primary hover:underline">
        全部错误码
      </NuxtLink>
      。
    </p>
  </div>
</template>
