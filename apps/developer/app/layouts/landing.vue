<script setup lang="ts">
const auth = useAuth()

await callOnce('auth:user', async () => {
  if (!auth.user.value) {
    await auth.fetchUser()
  }
})
</script>

<template>
  <div class="bg-background flex min-h-screen flex-col">
    <LayoutHeader />

    <!--
      No max-width and no padding on <main>, unlike the default layout: the
      landing hero is a full-bleed stage and each section re-applies max-w-7xl
      itself. Wrapping it in the default container instead and breaking out with
      negative margins reintroduces the horizontal scrollbar this repo reserves
      the gutter to avoid.
    -->
    <main class="w-full min-w-0 flex-1">
      <slot />
    </main>

    <LayoutFooter />
  </div>
</template>
