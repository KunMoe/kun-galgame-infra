<script setup lang="ts">
definePageMeta({ layout: 'docs' })

const route = useRoute()
const name = computed(() => String(route.params.name || ''))
const { vocabularies } = await import('~/generated/vocabularies')
const { vocabularyMetaZh, vocabularyValuesZh } =
  await import('~/generated/vocabularies-zh')
const entry = computed(() => vocabularies.find((v) => v.name === name.value))

if (!entry.value) {
  throw createError({
    statusCode: 404,
    statusMessage: 'Unknown vocabulary',
    fatal: true
  })
}

const meta = computed(() => vocabularyMetaZh[name.value])
const zh = computed(() => vocabularyValuesZh[name.value] ?? {})

useSeoMeta({
  title: () => `${meta.value?.label ?? entry.value!.name} · 词表`,
  description: () => meta.value?.summary ?? `Tokens for ${entry.value!.name}`
})
</script>

<template>
  <div v-if="entry" class="space-y-8">
    <nav class="text-default-400 flex items-center gap-1.5 text-sm">
      <NuxtLink to="/docs" class="hover:text-foreground">文档</NuxtLink>
      <KunIcon name="lucide:chevron-right" class="size-3.5" />
      <NuxtLink to="/docs/vocabularies" class="hover:text-foreground">
        词表
      </NuxtLink>
      <KunIcon name="lucide:chevron-right" class="size-3.5" />
      <span class="font-mono">{{ entry.name }}</span>
    </nav>

    <header>
      <h1 class="text-foreground text-2xl font-bold">
        {{ meta?.label ?? entry.name }}
      </h1>
      <div class="mt-3 flex flex-wrap items-center gap-2 text-xs">
        <span
          class="border-default-200 bg-content1 text-default-500 rounded-md border px-2 py-1 font-mono"
        >
          {{ entry.name }}
        </span>
        <span
          class="rounded-md px-2 py-1 font-medium"
          :class="
            entry.closed
              ? 'bg-success-50 text-success-600'
              : 'bg-warning-50 text-warning-600'
          "
        >
          {{ entry.closed ? '封闭词表' : '开放词表' }}
        </span>
        <span
          class="border-default-200 bg-content1 text-default-500 rounded-md border px-2 py-1"
        >
          {{ entry.values.length }} 个成员
        </span>
      </div>
      <p class="text-default-500 mt-4 max-w-2xl text-sm leading-relaxed">
        {{ meta?.summary }}
      </p>
      <p class="text-default-400 mt-2 max-w-2xl text-sm leading-relaxed">
        <template v-if="entry.closed">
          成员集合已经封定：加或减一个成员都算破坏性变更，会走大版本。
        </template>
        <template v-else>
          成员会继续增加：请把没见过的取值当作合法值容忍下来，不要断言这就是全集。
        </template>
        运行时以
        <code class="font-mono text-xs">GET /v2/vocabularies</code>
        为准，它比本页更新。
      </p>
    </header>

    <ul
      class="divide-default-200 border-default-200 bg-content1 divide-y overflow-hidden rounded-xl border"
    >
      <li v-for="val in entry.values" :key="val.value" class="px-4 py-3.5">
        <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <code class="text-primary font-mono text-sm">{{ val.value }}</code>
          <span class="text-foreground text-sm font-medium">
            {{ zh[val.value]?.displayName ?? val.display_name }}
          </span>
        </div>
        <p class="text-default-500 mt-1 text-sm leading-relaxed">
          {{ zh[val.value]?.description ?? val.description }}
        </p>
        <p class="text-default-300 mt-1 text-xs leading-relaxed">
          {{ val.display_name }} · {{ val.description }}
        </p>
      </li>
    </ul>

    <!--
      display_name and description are fields of the Value schema — they come
      back in the live response — so the English originals stay on the page
      instead of only in the Markdown twin. A caller diffing a response against
      this page has to be able to find the exact string it received.
    -->
    <p class="text-default-400 text-xs">
      每行第三行的灰色小字，是该成员在 API 响应里的英文
      <code class="font-mono">display_name</code>
      与
      <code class="font-mono">description</code>
      原文。纯英文的一页见
      <NuxtLink
        :to="`/docs/vocabularies/${entry.name}.md`"
        external
        class="text-primary hover:underline"
      >
        {{ `/docs/vocabularies/${entry.name}.md` }}
      </NuxtLink>
      。
    </p>
  </div>
</template>
