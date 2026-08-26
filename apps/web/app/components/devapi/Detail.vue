<script setup lang="ts">
import { DEV_TIER_COLORS, devTierLimitHint } from '~/constants/devapi'
import type { DevApp, DevKey, DevKeyMinted } from '~~/shared/types/devapi'

const route = useRoute()
const clientId = computed(() => String(route.params.clientId))

const api = useApi()

const { data: appsData, refresh: refreshApps } =
  await useApiFetch<DevApp[]>('/admin/devapi/apps')
const { data: keysData, refresh: refreshKeys } = await useApiFetch<DevKey[]>(
  () => `/admin/devapi/apps/${clientId.value}/keys`
)

const app = computed(() =>
  (appsData.value ?? []).find((a) => a.client_id === clientId.value) ?? null
)
const keys = computed(() => keysData.value ?? [])

const busy = ref(false)
const showConfigModal = ref(false)
const showMintModal = ref(false)

const mintedKey = ref<DevKeyMinted | null>(null)
const revealRotated = ref(false)
const revealOpen = ref(false)

const confirmDialog = ref<{
  title: string
  body: string
  danger?: boolean
  run: () => Promise<void>
} | null>(null)
const confirmOpen = ref(false)

const runConfirm = async () => {
  if (!confirmDialog.value) return
  busy.value = true
  try {
    await confirmDialog.value.run()
  } finally {
    busy.value = false
    confirmOpen.value = false
  }
}

const handleMinted = (minted: DevKeyMinted) => {
  showMintModal.value = false
  revealRotated.value = false
  mintedKey.value = minted
  revealOpen.value = true
  refreshKeys()
}

const askRotate = (key: DevKey) => {
  confirmOpen.value = true
  confirmDialog.value = {
    title: '轮换密钥',
    body: `将为「${key.name}」生成一枚新密钥，旧密钥进入 72 小时宽限后失效。请确保有机会保存新密钥。`,
    run: async () => {
      const res = await api.post<DevKeyMinted>(
        `/admin/devapi/apps/${clientId.value}/keys/${key.id}/rotate`
      )
      if (res.code === 0 && res.data) {
        revealRotated.value = true
        mintedKey.value = res.data
        revealOpen.value = true
        refreshKeys()
      } else {
        useKunMessage(res.message || '轮换失败', 'error')
      }
    },
  }
}

const askRevoke = (key: DevKey) => {
  confirmOpen.value = true
  confirmDialog.value = {
    title: '吊销密钥',
    body: `吊销「${key.name}」后立即生效且不可撤销，使用该密钥的所有请求将被拒绝。确认继续？`,
    danger: true,
    run: async () => {
      const res = await api.delete(
        `/admin/devapi/apps/${clientId.value}/keys/${key.id}`
      )
      if (res.code === 0) {
        useKunMessage('密钥已吊销', 'success')
        refreshKeys()
      } else {
        useKunMessage(res.message || '吊销失败', 'error')
      }
    },
  }
}

const askDisable = () => {
  confirmOpen.value = true
  confirmDialog.value = {
    title: '停用应用',
    body: '停用后该应用退出开放 API 平台，其所有密钥将立即失效。可随时重新启用。确认继续？',
    danger: true,
    run: async () => {
      const res = await api.patch(`/admin/devapi/apps/${clientId.value}`, {
        dev_enabled: false,
      })
      if (res.code === 0) {
        useKunMessage('应用已停用', 'success')
        navigateTo('/devapi')
      } else {
        useKunMessage(res.message || '停用失败', 'error')
      }
    },
  }
}

const handleConfigUpdated = () => {
  showConfigModal.value = false
  refreshApps()
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center gap-2">
      <KunButton variant="light" size="sm" is-icon-only aria-label="返回" @click="navigateTo('/devapi')">
        <KunIcon name="lucide:arrow-left" class="size-5" />
      </KunButton>
      <h1 class="text-2xl font-bold text-foreground">应用详情</h1>
    </div>

    <KunCard v-if="!app" content-class="p-10">
      <p class="text-center text-default-400">
        未找到该应用（可能已停用或不存在）。
      </p>
    </KunCard>

    <template v-else>
      <KunCard content-class="justify-start gap-0" class-name="p-6">
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h2 class="text-lg font-semibold text-foreground">{{ app.name }}</h2>
              <KunChip :color="DEV_TIER_COLORS[app.dev_tier] ?? 'default'" variant="flat" size="sm">
                {{ app.dev_tier }}
              </KunChip>
            </div>
            <div class="mt-1 flex items-center gap-2">
              <p class="truncate font-mono text-sm text-default-400">{{ app.client_id }}</p>
              <KunCopy :text="app.client_id" size="sm" />
            </div>
          </div>
          <div class="flex shrink-0 gap-1">
            <KunButton variant="flat" size="sm" @click="showConfigModal = true">
              <KunIcon name="lucide:sliders-horizontal" class="mr-1 size-4" />
              编辑配置
            </KunButton>
            <KunButton color="danger" variant="flat" size="sm" @click="askDisable">
              <KunIcon name="lucide:power-off" class="mr-1 size-4" />
              停用
            </KunButton>
          </div>
        </div>

        <div class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div>
            <p class="text-xs text-default-400">归属用户</p>
            <p class="mt-0.5 text-sm text-foreground">
              {{ app.owner_user_id ? `#${app.owner_user_id}` : '未指定' }}
            </p>
          </div>
          <div>
            <p class="text-xs text-default-400">限流</p>
            <p class="mt-0.5 text-sm text-foreground">
              {{ app.dev_rate_per_min > 0 ? `${app.dev_rate_per_min} 次/分` : '默认' }}
            </p>
          </div>
          <div>
            <p class="text-xs text-default-400">日配额</p>
            <p class="mt-0.5 text-sm text-foreground">
              {{ app.dev_quota_daily > 0 ? `${app.dev_quota_daily} 次/日` : '默认' }}
            </p>
          </div>
          <div>
            <p class="text-xs text-default-400">Tier 默认限额</p>
            <p class="mt-0.5 text-sm text-foreground">{{ devTierLimitHint(app.dev_tier) }}</p>
          </div>
        </div>
      </KunCard>

      <div class="flex items-center justify-between">
        <h2 class="text-lg font-semibold text-foreground">API 密钥</h2>
        <KunButton color="primary" size="sm" @click="showMintModal = true">
          <KunIcon name="lucide:plus" class="mr-1 size-4" />
          生成新密钥
        </KunButton>
      </div>

      <DevapiKeyTable :keys="keys" :busy="busy" @rotate="askRotate" @revoke="askRevoke" />
    </template>

    <DevapiConfigModal
      v-model:open="showConfigModal"
      :app="app"
      @updated="handleConfigUpdated"
    />

    <DevapiMintModal
      v-model:open="showMintModal"
      :client-id="clientId"
      @minted="handleMinted"
    />

    <DevapiKeyRevealModal
      v-model:open="revealOpen"
      :minted="mintedKey"
      :rotated="revealRotated"
    />

    <KunModal
      v-model="confirmOpen"
      role="alertdialog"
      :aria-label="confirmDialog?.title ?? '确认'"
    >
      <div class="space-y-4">
        <h2 class="text-xl font-bold text-foreground">{{ confirmDialog?.title }}</h2>
        <p class="text-sm text-default-500">{{ confirmDialog?.body }}</p>
        <div class="flex justify-end gap-3">
          <KunButton color="default" variant="flat" :disabled="busy" @click="confirmOpen = false">
            取消
          </KunButton>
          <KunButton :color="confirmDialog?.danger ? 'danger' : 'primary'" :disabled="busy" @click="runConfirm">
            <KunIcon v-if="busy" name="lucide:loader-circle" class="mr-2 size-4 animate-spin" />
            确认
          </KunButton>
        </div>
      </div>
    </KunModal>
  </div>
</template>
