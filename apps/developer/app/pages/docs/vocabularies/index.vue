<script setup lang="ts">
definePageMeta({ layout: 'docs' })

const { vocabularies } = await import('~/generated/vocabularies')

useSeoMeta({
  title: 'Vocabularies',
  description: 'Open and closed vocabularies published by NextMoe API v2.'
})
</script>

<template>
  <div class="space-y-6">
    <header>
      <h1 class="text-2xl font-bold text-foreground">Vocabularies</h1>
      <p class="mt-2 max-w-2xl text-sm text-default-500">
        Same members as
        <code class="font-mono text-xs">GET /v2/vocabularies</code>.
        Closed vocabularies cannot gain or lose a token without a breaking change.
      </p>
    </header>
    <ul class="divide-y divide-default-200 rounded-xl border border-default-200 bg-content1">
      <li v-for="v in vocabularies" :key="v.name">
        <NuxtLink
          :to="`/docs/vocabularies/${v.name}`"
          class="flex items-center justify-between gap-3 px-4 py-3 hover:bg-default-50"
        >
          <span class="font-mono text-sm text-foreground">{{ v.name }}</span>
          <span class="text-xs text-default-400">
            {{ v.closed ? 'closed' : 'open' }} · {{ v.values.length }}
          </span>
        </NuxtLink>
      </li>
    </ul>
  </div>
</template>
