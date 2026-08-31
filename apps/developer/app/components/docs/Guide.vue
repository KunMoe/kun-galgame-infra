<script setup lang="ts">
import { guideNav, guideMeta } from '~/generated/guides-nav'
import type { GuidePage } from '~~/shared/types/guides'

const props = defineProps<{ slug: string }>()

// One chunk per guide: a page ships its own prose and nothing else. The HTML is
// already Shiki-highlighted by scripts/build-guides.mjs, so neither markdown-it
// nor Shiki reaches the client bundle.
const modules = import.meta.glob<{ default: GuidePage }>(
  '../../generated/guides/*.json'
)
const loaders: Record<string, () => Promise<{ default: GuidePage }>> = {}
for (const [path, loader] of Object.entries(modules)) {
  loaders[
    path
      .split('/')
      .pop()!
      .replace(/\.json$/, '')
  ] = loader
}

const loader = loaders[props.slug]
if (!loader) {
  throw createError({
    statusCode: 404,
    statusMessage: `未找到文档 ${props.slug}`,
    fatal: true
  })
}

const { data: doc } = await useAsyncData(
  `guide:${props.slug}`,
  async () => (await loader()).default
)

const meta = guideMeta[`/docs/${props.slug}`]
useSeoMeta({
  title: () => meta?.title ?? '文档',
  description: () => meta?.description ?? ''
})

const flat = guideNav.flatMap((section) =>
  section.links.map((link) => ({ ...link, section: section.label }))
)
const index = computed(() =>
  flat.findIndex((l) => l.to === `/docs/${props.slug}`)
)
const prev = computed(() => (index.value > 0 ? flat[index.value - 1] : null))
const next = computed(() =>
  index.value >= 0 && index.value < flat.length - 1
    ? flat[index.value + 1]
    : null
)
</script>

<template>
  <div class="flex gap-10">
    <article class="kun-prose min-w-0 flex-1">
      <nav
        class="text-default-400 mb-6 flex flex-wrap items-center gap-1.5 text-sm"
      >
        <NuxtLink to="/docs" class="hover:text-primary transition-colors">
          文档
        </NuxtLink>
        <span>/</span>
        <span class="text-default-500">{{ meta?.eyebrow }}</span>
        <span>/</span>
        <span class="text-foreground">{{ meta?.title }}</span>
      </nav>

      <!-- eslint-disable-next-line vue/no-v-html -->
      <div v-html="doc?.html" />

      <div v-if="prev || next" class="mt-12 grid gap-3 sm:grid-cols-2">
        <NuxtLink
          v-if="prev"
          :to="prev.to"
          class="border-default-200 hover:border-primary rounded-xl border p-3 text-sm transition-colors"
        >
          <span class="text-default-400 block text-xs">← 上一篇</span>
          <span class="text-foreground">{{ prev.label }}</span>
        </NuxtLink>
        <span v-else class="hidden sm:block" />
        <NuxtLink
          v-if="next"
          :to="next.to"
          class="border-default-200 hover:border-primary rounded-xl border p-3 text-right text-sm transition-colors sm:col-start-2"
        >
          <span class="text-default-400 block text-xs">下一篇 →</span>
          <span class="text-foreground">{{ next.label }}</span>
        </NuxtLink>
      </div>
    </article>

    <aside
      v-if="doc?.toc?.length"
      class="sticky top-20 hidden h-fit w-52 shrink-0 xl:block"
    >
      <p
        class="text-default-400 mb-2 text-xs font-semibold tracking-wide uppercase"
      >
        本页目录
      </p>
      <ul class="flex flex-col gap-1 text-sm">
        <li
          v-for="h in doc.toc"
          :key="h.id"
          :class="h.depth === 3 ? 'pl-3' : ''"
        >
          <a
            :href="`#${h.id}`"
            class="text-default-500 hover:text-primary transition-colors"
          >
            {{ h.text }}
          </a>
        </li>
      </ul>
    </aside>
  </div>
</template>
