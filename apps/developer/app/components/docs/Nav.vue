<script setup lang="ts">
import { cn } from '@kungal/ui-core'
import { guideNav } from '~/generated/guides-nav'
import { DOCS_REFERENCE_NAV } from '~/constants/docs'
import type { DocsOperation } from '~~/shared/types/docs'

const route = useRoute()
const { faces, faceOperationCount } = useDocs()
const { t } = useDocsI18n()

const query = ref('')

const referenceFace = faces[0]!
const referenceCount = faceOperationCount(referenceFace)
const onReference = computed(() =>
  route.path.startsWith(`/docs/${referenceFace.key}`)
)

const matches = (op: DocsOperation): boolean => {
  const q = query.value.trim().toLowerCase()
  if (!q) return true
  return (
    op.path.toLowerCase().includes(q) ||
    op.id.toLowerCase().includes(q) ||
    op.summary.toLowerCase().includes(q) ||
    t(op.summary).toLowerCase().includes(q)
  )
}

const visibleGroups = computed(() =>
  referenceFace.groups
    .map((g) => ({ ...g, operations: g.operations.filter(matches) }))
    .filter((g) => g.operations.length > 0)
)
const activeOpId = computed(
  () => route.params.operationId as string | undefined
)

const isActive = (to: string) => route.path === to

const linkClass = (to: string) =>
  cn(
    'block rounded-lg px-2 py-1.5 text-sm transition-colors',
    isActive(to)
      ? 'bg-primary-50 font-medium text-primary'
      : 'text-default-500 hover:bg-default-100 hover:text-foreground'
  )
</script>

<template>
  <nav class="space-y-6">
    <NuxtLink to="/docs" :class="linkClass('/docs')">
      <span class="flex items-center gap-2">
        <KunIcon name="lucide:compass" class="size-4" />
        概览
      </span>
    </NuxtLink>

    <div v-for="section in guideNav" :key="section.key" class="space-y-1">
      <p class="text-default-400 px-2 text-xs font-semibold tracking-wide">
        {{ section.label }}
      </p>
      <NuxtLink
        v-for="link in section.links"
        :key="link.to"
        :to="link.to"
        :class="linkClass(link.to)"
      >
        {{ link.label }}
      </NuxtLink>
    </div>

    <div class="space-y-1">
      <p class="text-default-400 px-2 text-xs font-semibold tracking-wide">
        参考
      </p>
      <NuxtLink
        :to="`/docs/${referenceFace.key}`"
        :class="
          cn(
            'flex items-center justify-between gap-2 rounded-lg px-2 py-1.5 text-sm transition-colors',
            onReference
              ? 'bg-primary-50 text-primary font-medium'
              : 'text-default-500 hover:bg-default-100 hover:text-foreground'
          )
        "
      >
        <span>端点参考</span>
        <span
          class="bg-default-100 text-default-500 rounded-full px-1.5 text-xs"
        >
          {{ referenceCount }}
        </span>
      </NuxtLink>

      <div v-if="onReference" class="space-y-3 pt-1 pl-2">
        <div class="relative">
          <KunIcon
            name="lucide:search"
            class="text-default-300 pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2"
          />
          <input
            v-model="query"
            type="text"
            aria-label="过滤端点"
            placeholder="过滤端点…"
            class="border-default-200 bg-content1 text-foreground placeholder:text-default-300 focus:border-primary w-full rounded-lg border py-1.5 pr-2 pl-8 text-xs focus:outline-none"
          />
        </div>

        <div
          v-for="group in visibleGroups"
          :key="group.key"
          class="space-y-0.5"
        >
          <p class="text-default-400 px-1 text-xs">{{ group.label }}</p>
          <NuxtLink
            v-for="op in group.operations"
            :key="op.id"
            :to="`/docs/${referenceFace.key}/${op.id}`"
            :class="
              cn(
                'flex items-center gap-2 rounded-lg px-1.5 py-1 transition-colors',
                activeOpId === op.id
                  ? 'bg-primary-50 text-primary'
                  : 'text-default-500 hover:bg-default-100 hover:text-foreground'
              )
            "
          >
            <DocsMethodBadge :method="op.method" size="sm" />
            <code class="truncate font-mono text-xs">{{ op.path }}</code>
          </NuxtLink>
        </div>

        <p v-if="!visibleGroups.length" class="text-default-400 px-1 text-xs">
          没有匹配「{{ query }}」的端点
        </p>
      </div>

      <NuxtLink
        v-for="link in DOCS_REFERENCE_NAV"
        :key="link.to"
        :to="link.to"
        :class="linkClass(link.to)"
      >
        {{ link.label }}
      </NuxtLink>
    </div>
  </nav>
</template>
