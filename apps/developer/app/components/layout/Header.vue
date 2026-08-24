<script setup lang="ts">
const auth = useAuth()
const route = useRoute()
const colorMode = useColorMode()
const { open: openLogin } = useLoginModal()

const navLinks = [
  { to: '/', label: '首页' },
  { to: '/docs', label: 'API 文档' },
  { to: '/explore', label: '数据浏览' },
  { to: '/dashboard', label: '控制台' }
]

const isActive = (to: string) =>
  to === '/' ? route.path === '/' : route.path.startsWith(to)

const colorModeOptions = [
  { value: 'light', label: '浅色', icon: 'lucide:sun' },
  { value: 'dark', label: '深色', icon: 'lucide:moon' },
  { value: 'system', label: '跟随系统', icon: 'lucide:monitor' }
] as const

const setColorMode = (mode: string) => {
  colorMode.preference = mode
}

const handleLogout = async () => {
  await auth.logout()
}
</script>

<template>
  <header
    class="sticky top-0 z-30 border-b border-default-200 bg-background/85 backdrop-blur-md"
  >
    <div class="border-b border-warning-200 bg-warning-50 px-4 py-1.5 text-center text-xs text-warning-700">
      Public API v2 is
      <strong>preview</strong>
      · breaking changes allowed · third parties stay on /v1 ·
      <NuxtLink to="/docs/v2" class="underline">v2 docs</NuxtLink>
      ·
      <NuxtLink to="/docs/design" class="underline">design principles</NuxtLink>
    </div>
    <div
      class="mx-auto flex h-16 max-w-7xl items-center justify-between gap-4 px-4 md:px-6"
    >
      <NuxtLink to="/" class="flex min-w-0 shrink-0 items-center gap-2">
        <div
          class="flex size-8 items-center justify-center rounded-lg bg-primary text-white"
        >
          <KunIcon name="lucide:boxes" class="size-5" />
        </div>
        <span class="truncate text-base font-bold text-foreground">
          NextMoe 开发者平台
        </span>
      </NuxtLink>

      <nav class="hidden items-center gap-1 md:flex">
        <NuxtLink
          v-for="link in navLinks"
          :key="link.to"
          :to="link.to"
          class="rounded-lg px-3 py-2 text-sm font-medium transition-colors"
          :class="
            isActive(link.to)
              ? 'bg-primary-50 text-primary'
              : 'text-default-500 hover:bg-default-100 hover:text-foreground'
          "
        >
          {{ link.label }}
        </NuxtLink>
      </nav>

      <div class="flex shrink-0 items-center gap-2">
        <KunPopover position="bottom-end">
          <template #trigger>
            <KunButton
              variant="light"
              size="sm"
              is-icon-only
              aria-label="切换主题"
            >
              <KunIcon name="lucide:sun-moon" class="size-5" />
            </KunButton>
          </template>
          <div class="w-36 py-1">
            <button
              v-for="option in colorModeOptions"
              :key="option.value"
              class="flex w-full items-center gap-3 px-3 py-2 text-sm transition-colors"
              :class="
                colorMode.preference === option.value
                  ? 'bg-primary-50 text-primary'
                  : 'text-default-500 hover:bg-default-100 hover:text-foreground'
              "
              @click="setColorMode(option.value)"
            >
              <KunIcon :name="option.icon" class="size-4" />
              <span>{{ option.label }}</span>
            </button>
          </div>
        </KunPopover>

        <KunPopover v-if="auth.isLoggedIn.value" position="bottom-end">
          <template #trigger>
            <KunButton variant="flat" size="sm">
              <KunIcon name="lucide:circle-user-round" class="mr-1 size-4" />
              <span class="max-w-24 truncate">{{ auth.user.value?.name }}</span>
            </KunButton>
          </template>
          <div class="w-44 py-1">
            <NuxtLink
              to="/dashboard"
              class="flex w-full items-center gap-3 px-3 py-2 text-sm text-default-500 transition-colors hover:bg-default-100 hover:text-foreground md:hidden"
            >
              <KunIcon name="lucide:layout-dashboard" class="size-4" />
              <span>控制台</span>
            </NuxtLink>
            <button
              class="flex w-full items-center gap-3 px-3 py-2 text-sm text-danger transition-colors hover:bg-danger-50"
              @click="handleLogout"
            >
              <KunIcon name="lucide:log-out" class="size-4" />
              <span>退出登录</span>
            </button>
          </div>
        </KunPopover>

        <KunButton v-else color="primary" size="sm" @click="openLogin()">
          登录
        </KunButton>
      </div>
    </div>
  </header>
</template>
