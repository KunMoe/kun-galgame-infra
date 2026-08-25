<script setup lang="ts">
definePageMeta({ layout: 'docs' })

const route = useRoute()
const name = computed(() => String(route.params.name || ''))
const { vocabularies } = await import('~/generated/vocabularies')
const entry = computed(() => vocabularies.find((v) => v.name === name.value))

if (!entry.value) {
  throw createError({ statusCode: 404, statusMessage: 'Unknown vocabulary', fatal: true })
}

useSeoMeta({
  title: () => `${entry.value!.name} · vocabulary`,
  description: () => `Tokens for ${entry.value!.name}`
})
</script>

<template>
  <div v-if="entry" class="space-y-6">
    <nav class="flex items-center gap-1.5 text-sm text-default-400">
      <NuxtLink to="/docs/vocabularies" class="hover:text-foreground">Vocabularies</NuxtLink>
      <KunIcon name="lucide:chevron-right" class="size-3.5" />
      <span>{{ entry.name }}</span>
    </nav>
    <header>
      <h1 class="font-mono text-2xl font-bold text-foreground">{{ entry.name }}</h1>
      <p class="mt-2 text-sm text-default-500">
        {{ entry.closed ? 'Closed' : 'Open' }} · {{ entry.values.length }} tokens
      </p>
    </header>
    <ul class="divide-y divide-default-200 rounded-xl border border-default-200 bg-content1">
      <li v-for="val in entry.values" :key="val.value" class="px-4 py-3">
        <p class="font-mono text-sm text-foreground">{{ val.value }}</p>
        <p class="text-sm text-default-500">{{ val.display_name }}</p>
        <p class="text-xs text-default-400">{{ val.description }}</p>
      </li>
    </ul>
  </div>
</template>
