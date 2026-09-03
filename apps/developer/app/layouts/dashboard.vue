<script setup lang="ts">
import { DASHBOARD_NAV, isDashboardNavActive } from '~/constants/nav'

const auth = useAuth()
const route = useRoute()

const handleLogout = async () => {
  await auth.logout()
}
</script>

<template>
  <div class="bg-background flex min-h-screen flex-col">
    <LayoutHeader />
    <div
      class="mx-auto flex w-full max-w-7xl flex-1 flex-col px-4 md:px-6 lg:flex-row lg:gap-8"
    >
      <aside class="hidden shrink-0 py-10 lg:block lg:w-60">
        <div class="lg:sticky lg:top-24 lg:space-y-6">
          <div>
            <p
              class="text-default-400 px-3 text-xs font-semibold tracking-wider uppercase"
            >
              控制台
            </p>
            <nav class="mt-2 space-y-1">
              <NuxtLink
                v-for="item in DASHBOARD_NAV"
                :key="item.to"
                :to="item.to"
                class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors"
                :class="
                  isDashboardNavActive(item, route.path)
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
            class="border-default-200 bg-content1 rounded-xl border p-3"
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
                <p class="text-foreground truncate text-sm font-semibold">
                  {{ auth.user.value.name }}
                </p>
                <p class="text-default-400 truncate text-xs">
                  {{ auth.user.value.email }}
                </p>
              </div>
            </div>
            <button
              class="text-danger hover:bg-danger-50 mt-3 flex w-full items-center justify-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors"
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
