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

    <LayoutFooter />
  </div>
</template>
