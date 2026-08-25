<script setup lang="ts">
definePageMeta({ layout: 'docs' })

const { problems } = await import('~/generated/problems')

useSeoMeta({
  title: 'Problem types',
  description: 'RFC 9457 problem type catalog for NextMoe API v2.'
})

const grouped = computed(() => {
  const map = new Map<string, typeof problems>()
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
      <h1 class="text-2xl font-bold text-foreground">Problem types</h1>
      <p class="mt-2 max-w-2xl text-sm text-default-500">
        Every v2 error
        <code class="font-mono text-xs">type</code>
        URI resolves here. Machine-facing strings stay English.
      </p>
    </header>
    <section v-for="[domain, rows] in grouped" :key="domain" class="space-y-3">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-default-400">
        {{ domain }}
      </h2>
      <ul class="divide-y divide-default-200 rounded-xl border border-default-200 bg-content1">
        <li v-for="p in rows" :key="p.code">
          <NuxtLink
            :to="`/problems/${p.domain}/${p.kebab}`"
            class="flex items-center justify-between gap-3 px-4 py-3 hover:bg-default-50"
          >
            <span class="font-mono text-sm text-foreground">{{ p.code }}</span>
            <span class="truncate text-sm text-default-500">{{ p.title }}</span>
            <span class="shrink-0 font-mono text-xs text-default-400">{{ p.status }}</span>
          </NuxtLink>
        </li>
      </ul>
    </section>
  </div>
</template>
