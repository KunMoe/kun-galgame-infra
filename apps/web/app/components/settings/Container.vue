<script setup lang="ts">
import type {
  SettingsOverview,
  SettingsAuditEntry,
  SettingsKeyView,
  SettingKind,
  SettingValue
} from '~~/shared/types/settings'
import { AUDIT_LIST_LIMIT } from '~/constants/settings'

const CONFLICT_MESSAGE = '该配置已被其他人修改,请刷新后重试'

const api = useApi()

const {
  data: overviewData,
  status,
  refresh: refreshOverview,
  error
} = await useApiFetch<SettingsOverview>('/admin/settings')

const {
  data: auditData,
  refresh: refreshAudit,
  error: auditError
} = await useApiFetch<SettingsAuditEntry[]>('/admin/settings/audit', {
  query: { limit: AUDIT_LIST_LIMIT }
})

const overview = computed(() => overviewData.value)
const auditEntries = computed(() => auditData.value ?? [])
const isLoading = computed(() => status.value === 'pending')
const writable = computed(() => overview.value?.writable ?? false)

const kinds = computed<Record<string, SettingKind>>(() => {
  const map: Record<string, SettingKind> = {}
  for (const domain of overview.value?.domains ?? []) {
    for (const row of domain.keys) {
      map[row.key] = row.kind
    }
  }
  return map
})

const editOpen = ref(false)
const resetOpen = ref(false)
const pendingRow = ref<SettingsKeyView | null>(null)
const submitting = ref(false)

const settingPath = (key: string) =>
  `/admin/settings/${encodeURIComponent(key)}`

const askEdit = (row: SettingsKeyView) => {
  pendingRow.value = row
  editOpen.value = true
}

const askReset = (row: SettingsKeyView) => {
  pendingRow.value = row
  resetOpen.value = true
}

const rebindPending = () => {
  const key = pendingRow.value?.key
  if (!key) return
  const next = overview.value?.domains
    .flatMap((domain) => domain.keys)
    .find((row) => row.key === key)
  if (next) pendingRow.value = next
}

const handleWriteError = async (message: string) => {
  useKunMessage(message || '操作失败', 'error')
  if (message === CONFLICT_MESSAGE) {
    await Promise.all([refreshOverview(), refreshAudit()])
    rebindPending()
  }
}

const saveOverride = async (value: SettingValue, note: string) => {
  const row = pendingRow.value
  if (!row) return

  submitting.value = true
  try {
    const body: Record<string, unknown> = { value, note }
    if (row.override != null) {
      body.version = row.override.version
    }
    const response = await api.put<SettingsKeyView>(settingPath(row.key), body)
    if (response.code === 0) {
      useKunMessage('已保存', 'success')
      editOpen.value = false
      await Promise.all([refreshOverview(), refreshAudit()])
    } else {
      await handleWriteError(response.message)
    }
  } finally {
    submitting.value = false
  }
}

const confirmReset = async (note: string) => {
  const row = pendingRow.value
  if (!row) return

  submitting.value = true
  try {
    const response = await api.delete<SettingsKeyView>(settingPath(row.key), {
      note
    })
    if (response.code === 0) {
      useKunMessage('已撤销覆盖', 'success')
      resetOpen.value = false
      await Promise.all([refreshOverview(), refreshAudit()])
    } else {
      await handleWriteError(response.message)
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-foreground text-2xl font-bold">配置中心</h1>
      <p class="text-default-500 mt-1">
        生效顺序是代码默认 → 环境变量地板 → 这里的覆盖值；30
        秒内生效；密钥与接线不在这里。
      </p>
    </div>

    <CommonFetchError v-if="error" @retry="refreshOverview" />

    <div v-else-if="isLoading" class="flex items-center justify-center py-12">
      <KunIcon
        name="lucide:loader-circle"
        class="text-primary size-8 animate-spin"
      />
    </div>

    <template v-else-if="overview">
      <SettingsDomainCard
        v-for="domain in overview.domains"
        :key="domain.name"
        :domain="domain"
        :writable="writable"
        @edit="askEdit"
        @reset="askReset"
      />

      <SettingsAuditList
        :entries="auditEntries"
        :has-error="Boolean(auditError)"
        :kinds="kinds"
        @retry="refreshAudit"
      />

      <SettingsEditModal
        v-model:open="editOpen"
        :row="pendingRow"
        :submitting="submitting"
        @save="saveOverride"
      />

      <SettingsResetModal
        v-model:open="resetOpen"
        :row="pendingRow"
        :submitting="submitting"
        @confirm="confirmReset"
      />
    </template>
  </div>
</template>
