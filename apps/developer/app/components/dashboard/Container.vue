<script setup lang="ts">
import {
  DEV_APP_REVIEW_COLORS,
  DEV_APP_REVIEW_LABELS,
  DEV_CAP_APP_CREATE,
  DEV_DISABLED_HINT,
  DEV_TIER_COLORS,
  MAX_APPS_PER_ACCOUNT
} from '~/constants/dev'
import type { DevApp, DevPolicies } from '~~/shared/types/dev'

useSeoMeta({ title: '控制台', robots: 'noindex' })

const { data: appsData, refresh } = await useApiFetch<DevApp[]>('/dev/apps')
const { data: policiesData } = await useApiFetch<DevPolicies>('/dev/policies')

const apps = computed(() => appsData.value ?? [])
const createMode = computed(
  () => policiesData.value?.[DEV_CAP_APP_CREATE] ?? 'self_service'
)
const createDisabled = computed(() => createMode.value === 'disabled')
const needsApproval = computed(() => createMode.value === 'approval')

const showCreate = ref(false)
const atLimit = computed(() => apps.value.length >= MAX_APPS_PER_ACCOUNT)
const cannotCreate = computed(() => atLimit.value || createDisabled.value)

const reviewChip = (app: DevApp) => {
  const status = app.review_status
  if (status !== 'pending' && status !== 'declined') return null
  return {
    label: DEV_APP_REVIEW_LABELS[status],
    color: DEV_APP_REVIEW_COLORS[status]
  }
}

const handleCreated = () => {
  refresh()
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 class="text-foreground text-2xl font-bold">我的应用</h1>
        <p class="text-default-500 mt-1 text-sm">
          管理你的开放 API 应用与密钥 · 每个账号最多
          {{ MAX_APPS_PER_ACCOUNT }} 个应用
        </p>
      </div>
      <KunButton
        color="primary"
        :disabled="cannotCreate"
        @click="showCreate = true"
      >
        <KunIcon name="lucide:plus" class="mr-1 size-4" />
        创建应用
      </KunButton>
    </div>

    <div
      v-if="createDisabled"
      class="bg-danger-50 text-danger rounded-lg p-3 text-sm"
    >
      <KunIcon name="lucide:info" class="mr-1 inline size-4" />
      {{ DEV_DISABLED_HINT }}：平台暂不接受新应用注册，已有应用不受影响。
    </div>

    <div
      v-else-if="needsApproval"
      class="bg-warning-50 text-warning rounded-lg p-3 text-sm"
    >
      <KunIcon name="lucide:info" class="mr-1 inline size-4" />
      新应用需经平台审核后才会启用，审核通过前无法铸造密钥。
    </div>

    <div
      v-if="atLimit"
      class="bg-warning-50 text-warning rounded-lg p-3 text-sm"
    >
      <KunIcon name="lucide:info" class="mr-1 inline size-4" />
      已达到应用数量上限（{{
        MAX_APPS_PER_ACCOUNT
      }}）。如需更多，请删除不再使用的应用以归还名额。
    </div>

    <div v-if="apps.length" class="grid gap-4 sm:grid-cols-2">
      <NuxtLink
        v-for="app in apps"
        :key="app.client_id"
        :to="`/dashboard/apps/`"
        class="group"
      >
        <KunCard
          :is-hoverable="true"
          content-class="justify-start gap-0 items-start"
          class-name="p-5 h-full"
        >
          <div class="flex w-full items-start justify-between gap-3">
            <div class="flex flex-wrap items-center gap-2">
              <h2 class="text-foreground text-base font-semibold">
                {{ app.name }}
              </h2>
              <KunChip
                :color="DEV_TIER_COLORS[app.tier] ?? 'default'"
                variant="flat"
                size="xs"
              >
                {{ app.tier }}
              </KunChip>
              <KunChip
                v-if="reviewChip(app)"
                :color="reviewChip(app)!.color"
                variant="flat"
                size="xs"
              >
                {{ reviewChip(app)!.label }}
              </KunChip>
            </div>
            <KunIcon
              name="lucide:arrow-up-right"
              class="text-default-300 group-hover:text-primary size-4 shrink-0 transition-colors"
            />
          </div>

          <p
            v-if="app.description"
            class="text-default-500 mt-1 line-clamp-2 text-sm"
          >
            {{ app.description }}
          </p>

          <p class="text-default-400 mt-3 truncate font-mono text-xs">
            {{ app.client_id }}
          </p>

          <div class="text-default-400 mt-3 flex items-center gap-4 text-xs">
            <span class="flex items-center gap-1">
              <KunIcon name="lucide:key-round" class="size-3.5" />
              {{ app.key_count }} 个密钥
            </span>
            <span class="flex items-center gap-1">
              <KunIcon name="lucide:calendar" class="size-3.5" />
              {{ formatDate(app.created_at, { isShowYear: true }) }}
            </span>
          </div>
        </KunCard>
      </NuxtLink>
    </div>

    <KunCard v-else content-class="p-12" class-name="border-dashed">
      <div class="text-center">
        <div
          class="bg-default-100 mx-auto flex size-12 items-center justify-center rounded-full"
        >
          <KunIcon name="lucide:boxes" class="text-default-400 size-6" />
        </div>
        <p class="text-foreground mt-4 font-medium">还没有应用</p>
        <p class="text-default-500 mt-1 text-sm">
          创建第一个应用,即可领取密钥并调用开放 API。
        </p>
        <KunButton
          color="primary"
          class="mt-4"
          :disabled="createDisabled"
          @click="showCreate = true"
        >
          <KunIcon name="lucide:plus" class="mr-1 size-4" />
          创建应用
        </KunButton>
      </div>
    </KunCard>

    <DashboardCreateAppModal
      v-model:open="showCreate"
      :needs-approval="needsApproval"
      @created="handleCreated"
    />
  </div>
</template>
