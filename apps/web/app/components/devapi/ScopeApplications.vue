<script setup lang="ts">
import {
  DEV_SCOPE_APP_REASON_MAX,
  DEV_SCOPE_APP_STATUS_COLORS,
  DEV_SCOPE_APP_STATUS_LABELS,
} from '~/constants/devapi'
import type { DevScopeApplication } from '~~/shared/types/devapi'

const api = useApi()

const showAll = ref(false)
const { data, status, refresh } = await useApiFetch<DevScopeApplication[]>(
  () => `/admin/devapi/scope-applications?status=${showAll.value ? 'all' : 'pending'}`
)

const applications = computed(() => data.value ?? [])
const isLoading = computed(() => status.value === 'pending')

const busyId = ref<number | null>(null)
// The application outlives the close so the panel still has a scope to render
// while KunModal's leave transition plays; `declineOpen` drives the dialog.
const declining = ref<DevScopeApplication | null>(null)
const declineOpen = ref(false)
const declineReason = ref('')
const declineError = ref('')

const approve = async (app: DevScopeApplication) => {
  busyId.value = app.id
  try {
    const res = await api.post(
      `/admin/devapi/scope-applications/${app.id}/approve`
    )
    if (res.code === 0) {
      useKunMessage(`已批准 #${app.user_id} 的 ${app.scope}`, 'success')
      refresh()
    } else {
      useKunMessage(res.message || '批准失败', 'error')
    }
  } finally {
    busyId.value = null
  }
}

const openDecline = (app: DevScopeApplication) => {
  declining.value = app
  declineReason.value = ''
  declineError.value = ''
  declineOpen.value = true
}

const submitDecline = async () => {
  const app = declining.value
  if (!app) return
  if (!declineReason.value.trim()) {
    declineError.value = '请填写拒绝理由（申请人会看到）'
    return
  }
  busyId.value = app.id
  try {
    const res = await api.post(
      `/admin/devapi/scope-applications/${app.id}/decline`,
      { reason: declineReason.value.trim() }
    )
    if (res.code === 0) {
      useKunMessage('已拒绝并回执理由', 'success')
      declineOpen.value = false
      refresh()
    } else {
      declineError.value = res.message || '拒绝失败'
    }
  } finally {
    busyId.value = null
  }
}
</script>

<template>
  <div class="space-y-3">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div>
        <h2 class="text-lg font-semibold text-foreground">Scope 申请审核</h2>
        <p class="mt-1 text-sm text-default-500">
          授权制 scope（news:read）的申请。批准后申请人即可自助为密钥勾选它。
        </p>
      </div>
      <KunButton variant="flat" size="sm" @click="showAll = !showAll">
        {{ showAll ? '只看待审核' : '看全部' }}
      </KunButton>
    </div>

    <div v-if="isLoading" class="flex items-center justify-center py-8">
      <KunIcon
        name="lucide:loader-circle"
        class="size-6 animate-spin text-primary"
      />
    </div>

    <div
      v-else-if="!applications.length"
      class="rounded-xl bg-content1 py-8 text-center text-sm text-default-400 shadow-sm"
    >
      {{ showAll ? '还没有任何 scope 申请' : '没有待审核的申请' }}
    </div>

    <div v-else class="space-y-3">
      <KunCard
        v-for="app in applications"
        :key="app.id"
        content-class="justify-start gap-0 items-stretch"
        class-name="p-5"
      >
        <div class="flex flex-wrap items-center gap-2">
          <code class="font-mono text-sm text-foreground">{{ app.scope }}</code>
          <KunChip
            :color="DEV_SCOPE_APP_STATUS_COLORS[app.status] ?? 'default'"
            variant="flat"
            size="xs"
          >
            {{ DEV_SCOPE_APP_STATUS_LABELS[app.status] ?? app.status }}
          </KunChip>
          <span class="text-xs text-default-400">
            用户 #{{ app.user_id }} ·
            {{ formatDate(app.created_at, { isShowYear: true, isPrecise: true }) }}
          </span>
        </div>

        <p
          class="mt-2 whitespace-pre-line text-sm leading-relaxed text-default-500"
        >
          {{ app.message }}
        </p>

        <p v-if="app.decline_reason" class="mt-2 text-sm text-danger">
          拒绝理由：{{ app.decline_reason }}
        </p>

        <div v-if="app.status === 'pending'" class="mt-3 flex justify-end gap-2">
          <KunButton
            color="danger"
            variant="flat"
            size="sm"
            :disabled="busyId === app.id"
            @click="openDecline(app)"
          >
            拒绝
          </KunButton>
          <KunButton
            color="primary"
            size="sm"
            :disabled="busyId === app.id"
            @click="approve(app)"
          >
            <KunIcon
              v-if="busyId === app.id"
              name="lucide:loader-circle"
              class="mr-1 size-4 animate-spin"
            />
            批准
          </KunButton>
        </div>
      </KunCard>
    </div>

    <KunModal
      v-model="declineOpen"
      size="md"
      :aria-label="declining ? `拒绝 ${declining.scope} 申请` : '拒绝申请'"
    >
      <div v-if="declining" class="space-y-4">
        <h2 class="text-xl font-bold text-foreground">
          拒绝 {{ declining.scope }} 申请
        </h2>
        <p class="text-sm text-default-500">
          理由会原样回执给申请人（用户 #{{ declining.user_id }}），并允许其修改后重新申请。
        </p>
        <KunTextarea
          v-model="declineReason"
          label="拒绝理由"
          placeholder="例如：用途未说明会如何展示来源与回源链接。"
          :rows="4"
          :maxlength="DEV_SCOPE_APP_REASON_MAX"
          show-char-count
        />
        <div
          v-if="declineError"
          class="rounded-lg bg-danger-50 p-3 text-sm text-danger"
        >
          {{ declineError }}
        </div>
        <div class="flex justify-end gap-3">
          <KunButton color="default" variant="flat" @click="declineOpen = false">
            取消
          </KunButton>
          <KunButton color="danger" @click="submitDecline">确认拒绝</KunButton>
        </div>
      </div>
    </KunModal>
  </div>
</template>
