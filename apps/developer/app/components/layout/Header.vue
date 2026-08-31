<script setup lang="ts">
import { cn } from '@kungal/ui-core'
import { SITE_NAV } from '~/constants/nav'

const auth = useAuth()
const route = useRoute()
const colorMode = useColorMode()
const { open: openLogin } = useLoginModal()

const scrolled = ref(false)

const syncScrolled = () => {
  scrolled.value = window.scrollY > 0
}

onMounted(() => {
  syncScrolled()
  window.addEventListener('scroll', syncScrolled, { passive: true })
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', syncScrolled)
})

watch(
  () => route.fullPath,
  async () => {
    await nextTick()
    if (import.meta.client) syncScrolled()
  }
)

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
</script>

<template>
  <header
    :class="
      cn(
        'sticky top-0 z-30 border-b transition-[background-color,border-color,backdrop-filter] duration-200',
        scrolled
          ? 'border-default-200 bg-background/85 backdrop-blur-md'
          : 'border-transparent bg-transparent'
      )
    "
  >
    <div
      class="mx-auto flex h-16 max-w-7xl items-center gap-2 px-4 md:gap-4 md:px-6"
    >
      <LayoutMobileNav />

      <NuxtLink to="/" class="flex min-w-0 shrink-0 items-center gap-2">
        <img
          src="/favicon.webp"
          alt="NextMoe"
          width="256"
          height="256"
          class="border-default-200 bg-content1 size-8 shrink-0 rounded-lg border object-cover"
        />
        <span
          class="text-foreground hidden truncate text-base font-bold sm:inline"
        >
          NextMoe 开发者平台
        </span>
      </NuxtLink>

      <nav class="hidden items-center gap-1 lg:flex">
        <NuxtLink
          v-for="link in SITE_NAV"
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

      <div class="ml-auto flex shrink-0 items-center gap-2">
        <LayoutSearch />

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

        <LayoutAccountMenu v-if="auth.isLoggedIn.value" />
        <KunButton v-else color="primary" size="sm" @click="openLogin()">
          登录
        </KunButton>
      </div>
    </div>
  </header>
</template>
