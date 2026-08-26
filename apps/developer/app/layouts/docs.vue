<script setup lang="ts">
</script>

<template>
  <div class="flex min-h-screen flex-col bg-background">
    <LayoutHeader />
    <div
      class="mx-auto flex w-full max-w-7xl flex-1 flex-col px-4 md:px-6 lg:flex-row lg:gap-8"
    >
      <DocsSidebar />
      <main class="min-w-0 flex-1 py-8 lg:py-10">
        <div class="mb-4 flex justify-end">
          <DocsCopyPage />
        </div>
        <slot />
      </main>
    </div>
    <LayoutFooter />
  </div>
</template>
