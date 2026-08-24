<script setup lang="ts">
definePageMeta({ layout: 'docs' })

const route = useRoute()
const domain = computed(() => String(route.params.domain || ''))
const kebab = computed(() => String(route.params.code || ''))

const { problems } = await import('~/generated/problems')

const entry = computed(() =>
  problems.find((p) => p.domain === domain.value && p.kebab === kebab.value)
)

if (!entry.value) {
  throw createError({ statusCode: 404, statusMessage: 'Unknown problem type', fatal: true })
}

useSeoMeta({
  title: () => `${entry.value!.code} · Problem`,
  description: () => entry.value!.description
})
</script>

<template>
  <div v-if="entry" class="space-y-6">
    <nav class="flex items-center gap-1.5 text-sm text-default-400">
      <NuxtLink to="/docs" class="hover:text-foreground">文档</NuxtLink>
      <KunIcon name="lucide:chevron-right" class="size-3.5" />
      <NuxtLink to="/problems" class="hover:text-foreground">Problems</NuxtLink>
      <KunIcon name="lucide:chevron-right" class="size-3.5" />
      <span class="text-default-500">{{ entry.code }}</span>
    </nav>
    <header>
      <p class="font-mono text-xs text-default-400">{{ entry.type }}</p>
      <h1 class="mt-2 text-2xl font-bold text-foreground">{{ entry.title }}</h1>
      <p class="mt-2 text-sm text-default-500">
        <code class="rounded bg-default-100 px-1 py-0.5 font-mono text-xs">{{ entry.code }}</code>
        · HTTP {{ entry.status }}
        · {{ entry.domain }}
      </p>
    </header>
    <p class="max-w-2xl text-sm leading-relaxed text-default-500">
      {{ entry.description }}
    </p>
  </div>
</template>
