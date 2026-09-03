<script setup lang="ts">
import type { DevLiveKey } from '~~/shared/types/dev'

defineProps<{ keys: DevLiveKey[] }>()

const fmt = (n: number) => n.toLocaleString()

const usedPct = (k: DevLiveKey) =>
  k.quota_limit > 0
    ? Math.min(100, Math.round((k.quota_used / k.quota_limit) * 100))
    : 0

const tone = (pct: number) =>
  pct >= 90 ? 'danger' : pct >= 70 ? 'warning' : 'primary'
</script>

<template>
  <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
    <div
      v-for="k in keys"
      :key="k.key_id"
      class="border-default-200 bg-content1 rounded-xl border px-5 py-4"
    >
      <div class="flex items-center justify-between gap-2">
        <span class="text-foreground truncate font-medium">
          {{ k.app_name }}
        </span>
        <KunChip color="default" variant="flat" size="xs">
          #{{ k.key_id }}
        </KunChip>
      </div>

      <template v-if="k.quota_limit > 0">
        <div class="mt-3 flex items-baseline justify-between text-sm">
          <span class="text-default-500">今日剩余</span>
          <span class="text-foreground font-semibold tabular-nums">
            {{ fmt(k.quota_remaining) }}
            <span class="text-default-400 text-xs font-normal">
              / {{ fmt(k.quota_limit) }}
            </span>
          </span>
        </div>
        <div
          class="bg-default-200 mt-2 h-1.5 w-full overflow-hidden rounded-full"
        >
          <div
            class="h-full rounded-full transition-all"
            :class="{
              'bg-primary': tone(usedPct(k)) === 'primary',
              'bg-warning': tone(usedPct(k)) === 'warning',
              'bg-danger': tone(usedPct(k)) === 'danger'
            }"
            :style="{ width: `${usedPct(k)}%` }"
          />
        </div>
        <p class="text-default-400 mt-2 text-xs">
          已用 {{ fmt(k.quota_used) }} · 速率上限 {{ fmt(k.rate_limit) }} 次/分
        </p>
      </template>

      <template v-else>
        <p class="text-default-500 mt-3 text-sm">配额无限制(内部层)</p>
        <p class="text-default-400 mt-1 text-xs">
          已用 {{ fmt(k.quota_used) }}
        </p>
      </template>
    </div>
  </div>
</template>
