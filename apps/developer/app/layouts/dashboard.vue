<script setup lang="ts">
const auth = useAuth()
const route = useRoute()

const navItems = [
  {
    to: '/dashboard',
    label: '我的应用',
    icon: 'lucide:boxes',
    match: ['/dashboard', '/apps']
  },
  {
    to: '/usage',
    label: '用量',
    icon: 'lucide:chart-column',
    match: ['/usage']
  },
  {
    to: '/store',
    label: '分销链接',
    icon: 'lucide:shopping-bag',
    match: ['/store']
  },
  {
    to: '/docs',
    label: 'API 文档',
    icon: 'lucide:book-open',
    match: ['/docs']
  }
]

const isActive = (item: (typeof navItems)[number]) =>
  item.match.some((m) => route.path === m || route.path.startsWith(`${m}/`))

const handleLogout = async () => {
  await auth.logout()
}
</script>

<template>
  <div class="flex min-h-screen flex-col bg-background">
    <LayoutHeader />
    <div
      class="mx-auto flex w-full max-w-7xl flex-1 flex-col px-4 md:px-6 lg:flex-row lg:gap-8"
    >
      <aside class="hidden shrink-0 py-10 lg:block lg:w-60">
        <div class="lg:sticky lg:top-24 lg:space-y-6">
          <div>
            <p
              class="px-3 text-xs font-semibold uppercase tracking-wider text-default-400"
            >
              控制台
            </p>
            <nav class="mt-2 space-y-1">
              <NuxtLink
                v-for="item in navItems"
                :key="item.to"
                :to="item.to"
                class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors"
                :class="
                  isActive(item)
                    ? 'bg-primary-50 text-primary'
                    : 'text-default-500 hover:bg-default-100 hover:text-foreground'
                "
              >
                <KunIcon :name="item.icon" class="size-4" />
                {{ item.label }}
              </NuxtLink>
            </nav>
          </div>

          <div
            v-if="auth.user.value"
            class="rounded-xl border border-default-200 bg-content1 p-3"
          >
            <div class="flex items-center gap-3">
              <KunAvatar
                :user="{
                  id: 0,
                  name: auth.user.value.name,
                  avatar: auth.user.value.avatar || ''
                }"
                size="md"
                :is-navigation="false"
              />
              <div class="min-w-0">
                <p class="truncate text-sm font-semibold text-foreground">
                  {{ auth.user.value.name }}
                </p>
                <p class="truncate text-xs text-default-400">
                  {{ auth.user.value.email }}
                </p>
              </div>
            </div>
            <button
              class="mt-3 flex w-full items-center justify-center gap-2 rounded-lg px-3 py-2 text-sm text-danger transition-colors hover:bg-danger-50"
              @click="handleLogout"
            >
              <KunIcon name="lucide:log-out" class="size-4" />
              退出登录
            </button>
          </div>
        </div>
      </aside>

      <main class="min-w-0 flex-1 py-6 lg:py-10">
        <slot />
      </main>
    </div>
  </div>
</template>
