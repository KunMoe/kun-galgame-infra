<script setup lang="ts">
import { cn } from '@kungal/ui-core'
import { DASHBOARD_NAV, SITE_NAV, isDashboardRoute } from '~/constants/nav'

const route = useRoute()
const open = ref(false)

const onDocs = computed(
  () => route.path.startsWith('/docs') || route.path.startsWith('/problems')
)
const onDashboard = computed(() => isDashboardRoute(route.path))

const isActive = (to: string) =>
  to === '/' ? route.path === '/' : route.path.startsWith(to)

watch(
  () => route.fullPath,
  () => {
    open.value = false
  }
)

const linkClass = (active: boolean) =>
  cn(
    'flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors',
    active
      ? 'bg-primary-50 text-primary'
      : 'text-default-500 hover:bg-default-100 hover:text-foreground'
  )
</script>

<template>
  <div class="lg:hidden">
    <KunButton
      variant="light"
      size="sm"
      is-icon-only
      aria-label="打开导航菜单"
      @click="open = true"
    >
      <KunIcon name="lucide:menu" class="size-5" />
    </KunButton>

    <!--
      responsive defaults to true, and that turns a drawer into a bottom sheet
      below 48rem — i.e. on exactly the phones this one exists for. It has to be
      off for `placement="left"` to mean left on a phone.
    -->
    <KunDrawer
      v-model="open"
      placement="left"
      size="sm"
      :responsive="false"
      title="导航"
      inner-class-name="max-w-[85vw]"
    >
      <div class="space-y-6">
        <nav class="space-y-1">
          <NuxtLink
            v-for="link in SITE_NAV"
            :key="link.to"
            :to="link.to"
            :class="linkClass(isActive(link.to))"
          >
            <KunIcon :name="link.icon" class="size-4 shrink-0" />
            {{ link.label }}
          </NuxtLink>
        </nav>

        <div
          v-if="onDashboard"
          class="border-default-200 space-y-1 border-t pt-5"
        >
          <p
            class="text-default-400 px-3 pb-1 text-xs font-semibold tracking-wide"
          >
            控制台
          </p>
          <NuxtLink
            v-for="item in DASHBOARD_NAV"
            :key="item.to"
            :to="item.to"
            :class="linkClass(isActive(item.to))"
          >
            <KunIcon :name="item.icon" class="size-4 shrink-0" />
            {{ item.label }}
          </NuxtLink>
        </div>

        <!--
          Lazy because DocsNav pulls in the 4.5 MB generated docs model. As
          `<LazyDocsNav>` it is a chunk of its own, so the landing page's header
          does not carry the whole API reference.
        -->
        <div v-if="onDocs" class="border-default-200 border-t pt-5">
          <LazyDocsNav />
        </div>
      </div>
    </KunDrawer>
  </div>
</template>
