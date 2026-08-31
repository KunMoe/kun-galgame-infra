<script setup lang="ts">
definePageMeta({ layout: 'docs' })

const { vocabularies } = await import('~/generated/vocabularies')
const { vocabularyMetaZh } = await import('~/generated/vocabularies-zh')

useSeoMeta({
  title: '词表 · Vocabularies',
  description:
    'NextMoe API v2 公布的全部开放与封闭词表：每个枚举的成员，以及它是开放还是封闭的。'
})

const closedCount = vocabularies.filter((v) => v.closed).length
</script>

<template>
  <div class="space-y-8">
    <header>
      <p class="text-primary text-sm font-medium">运行时发现</p>
      <h1 class="text-foreground mt-1 text-2xl font-bold">
        词表
        <span class="text-default-400 ml-2 font-mono text-base font-normal">
          Vocabularies
        </span>
      </h1>
      <p class="text-default-500 mt-3 max-w-2xl text-sm leading-relaxed">
        与
        <code class="font-mono text-xs">GET /v2/vocabularies</code>
        同一份成员，不需要凭据。
        <strong class="text-foreground font-semibold">封闭</strong>
        词表不加不减成员就是我们的承诺，你的
        <code class="font-mono text-xs">switch</code>
        可以没有
        <code class="font-mono text-xs">default</code>
        ；
        <strong class="text-foreground font-semibold">开放</strong>
        词表相反，它提前告诉你会有新值，客户端必须容忍没见过的取值——见
        <NuxtLink to="/docs/versioning" class="text-primary hover:underline">
          版本策略
        </NuxtLink>
        。下面是中文说明，
        <code class="font-mono text-xs">value</code>
        这类线上取值一律不翻译。
      </p>
      <p class="text-default-400 mt-2 text-xs">
        共 {{ vocabularies.length }} 个词表，其中封闭 {{ closedCount }} 个。
      </p>
    </header>

    <ul class="grid gap-3 sm:grid-cols-2">
      <li v-for="v in vocabularies" :key="v.name">
        <NuxtLink
          :to="`/docs/vocabularies/${v.name}`"
          class="border-default-200 bg-content1 hover:border-primary-200 hover:bg-default-50 flex h-full flex-col rounded-xl border p-4 transition-colors"
        >
          <div class="flex items-center gap-2">
            <span class="text-foreground text-sm font-semibold">
              {{ vocabularyMetaZh[v.name]?.label ?? v.name }}
            </span>
            <span
              class="rounded-md px-1.5 py-0.5 text-[0.6875rem] font-medium"
              :class="
                v.closed
                  ? 'bg-success-50 text-success-600'
                  : 'bg-warning-50 text-warning-600'
              "
            >
              {{ v.closed ? '封闭' : '开放' }}
            </span>
            <span class="text-default-400 ml-auto shrink-0 font-mono text-xs">
              {{ v.values.length }}
            </span>
          </div>
          <p class="text-default-400 mt-1 font-mono text-xs">{{ v.name }}</p>
          <p class="text-default-500 mt-2 text-sm leading-relaxed">
            {{ vocabularyMetaZh[v.name]?.summary }}
          </p>
        </NuxtLink>
      </li>
    </ul>
  </div>
</template>
