<script setup lang="ts">
const auth = useAuth()

await callOnce('auth:user', async () => {
  if (!auth.user.value) {
    await auth.fetchUser()
  }
})
</script>

<template>
  <div class="flex min-h-screen flex-col bg-background">
    <LayoutHeader />

    <main class="mx-auto w-full min-w-0 max-w-7xl flex-1 px-4 py-8 md:px-6">
      <slot />
    </main>

    <footer class="border-t border-default-200">
      <div
        class="mx-auto flex max-w-7xl flex-col items-center justify-between gap-2 px-4 py-6 text-sm text-default-400 md:flex-row md:px-6"
      >
        <p>NextMoe 开放 API · 六源对齐的 galgame 数据</p>
        <div class="flex items-center gap-3">
          <a href="/llms.txt" class="text-xs hover:text-foreground">llms.txt</a>
          <p class="font-mono text-xs">api.nextmoe.dev/v2</p>
        </div>
      </div>
    </footer>
  </div>
</template>
