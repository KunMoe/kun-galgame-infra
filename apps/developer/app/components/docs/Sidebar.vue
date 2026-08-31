<script setup lang="ts">
import { cn } from '@kungal/ui-core'
import type { DocsOperation } from '~~/shared/types/docs'

const route = useRoute()
const { faces } = useDocs()
const { t } = useDocsI18n()

const query = ref('')
const mobileOpen = ref(false)

const activeFaceKey = computed(
  () => (route.params.face as string) || faces[0]!.key
)
const activeFace = computed(
  () => faces.find((f) => f.key === activeFaceKey.value) ?? faces[0]!
)
const activeOpId = computed(
  () => route.params.operationId as string | undefined
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
  activeFace.value.groups
    .map((g) => ({ ...g, operations: g.operations.filter(matches) }))
    .filter((g) => g.operations.length > 0)
)
const showGroupHeaders = computed(() => activeFace.value.groups.length > 1)
const hasResults = computed(() => visibleGroups.value.length > 0)

watch(
  () => route.fullPath,
  () => {
    mobileOpen.value = false
  }
)
</script>

<template>
  <aside
    class="lg:border-default-200 lg:sticky lg:top-16 lg:h-[calc(100vh-4rem)] lg:w-64 lg:shrink-0 lg:overflow-y-auto lg:border-r"
  >
    <button
      type="button"
      class="border-default-200 text-foreground flex w-full items-center justify-between gap-2 border-b py-3 text-sm font-semibold lg:hidden"
      :aria-expanded="mobileOpen"
      @click="mobileOpen = !mobileOpen"
    >
      <span class="flex items-center gap-2">
        <KunIcon name="lucide:list" class="text-default-400 size-4" />
        API 目录
      </span>
      <KunIcon
        :name="mobileOpen ? 'lucide:chevron-up' : 'lucide:chevron-down'"
        class="text-default-400 size-4"
      />
    </button>

    <div
      :class="
        cn('space-y-4 py-4 lg:pr-3', mobileOpen ? 'block' : 'hidden lg:block')
      "
    >
      <div class="mb-3 space-y-1 text-xs">
        <NuxtLink
          to="/docs/design"
          class="text-default-400 hover:text-foreground block"
        >
          设计原则
        </NuxtLink>
        <NuxtLink
          to="/docs/vocabularies"
          class="text-default-400 hover:text-foreground block"
        >
          词表
        </NuxtLink>
        <NuxtLink
          to="/problems"
          class="text-default-400 hover:text-foreground block"
        >
          Problem types
        </NuxtLink>
      </div>
      <div class="bg-default-100 grid grid-cols-2 gap-1 rounded-lg p-1">
        <NuxtLink
          v-for="f in faces"
          :key="f.key"
          :to="`/docs/${f.key}`"
          :class="
            cn(
              'rounded-md px-2 py-1.5 text-center text-sm font-medium transition-colors',
              f.key === activeFaceKey
                ? 'bg-content1 text-foreground shadow-sm'
                : 'text-default-500 hover:text-foreground'
            )
          "
        >
          {{ f.label }}
        </NuxtLink>
      </div>

      <div class="relative">
        <KunIcon
          name="lucide:search"
          class="text-default-300 pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
        />
        <input
          v-model="query"
          type="text"
          aria-label="过滤端点"
          placeholder="过滤端点…"
          class="border-default-200 bg-content1 text-foreground placeholder:text-default-300 focus:border-primary w-full rounded-lg border py-2 pr-3 pl-9 text-sm focus:outline-none"
        />
      </div>

      <nav v-if="hasResults" class="space-y-4">
        <div v-for="group in visibleGroups" :key="group.key" class="space-y-1">
          <p
            v-if="showGroupHeaders"
            class="text-default-400 px-2 text-xs font-semibold tracking-wide"
          >
            {{ group.label }}
          </p>
          <ul class="space-y-0.5">
            <li v-for="op in group.operations" :key="op.id">
              <NuxtLink
                :to="`/docs/${activeFace.key}/${op.id}`"
                :class="
                  cn(
                    'flex items-center gap-2 rounded-lg px-2 py-1.5 transition-colors',
                    activeOpId === op.id
                      ? 'bg-primary-50 text-primary'
                      : 'text-default-500 hover:bg-default-100 hover:text-foreground'
                  )
                "
              >
                <DocsMethodBadge :method="op.method" size="sm" />
                <code class="truncate font-mono text-xs">{{ op.path }}</code>
              </NuxtLink>
            </li>
          </ul>
        </div>
      </nav>

      <p v-else class="text-default-400 px-2 text-sm">
        没有匹配「{{ query }}」的端点
      </p>
    </div>
  </aside>
</template>
