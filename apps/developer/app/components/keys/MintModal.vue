<script setup lang="ts">
import {
  DEV_DISABLED_HINT,
  DEV_GRANTABLE_SCOPES,
  DEV_MINTABLE_SCOPES,
  DEV_SCOPE_APP_STATUS_COLORS,
  DEV_SCOPE_APP_STATUS_LABELS
} from '~/constants/dev'
import type { DevKeyMinted, DevScopeApplication } from '~~/shared/types/dev'

const props = defineProps<{ clientId: string; scopeApplyDisabled?: boolean }>()
const emit = defineEmits<{ minted: [DevKeyMinted] }>()

const open = defineModel<boolean>('open', { required: true })
const api = useApi()

const name = ref('')
const test = ref(false)
const scopes = ref<string[]>([...DEV_MINTABLE_SCOPES])
const error = ref('')
const isLoading = ref(false)

const applications = ref<DevScopeApplication[]>([])
const applyScope = ref('')
const applyOpen = ref(false)

const loadApplications = async () => {
  const res = await api.get<DevScopeApplication[]>('/dev/scope-applications')
  applications.value = res.code === 0 && res.data ? res.data : []
}

const applicationFor = (scope: string) =>
  applications.value.find((a) => a.scope === scope) ?? null

const isGranted = (scope: string) => applicationFor(scope)?.status === 'approved'

const handleFiled = (filed: DevScopeApplication) => {
  applyOpen.value = false
  applications.value = [
    ...applications.value.filter((a) => a.scope !== filed.scope),
    filed
  ]
  useKunMessage('申请已提交，等待平台审核', 'success')
}

const openApply = (scope: string) => {
  applyScope.value = scope
  applyOpen.value = true
}

watch(open, (val) => {
  if (!val) return
  name.value = ''
  test.value = false
  scopes.value = [...DEV_MINTABLE_SCOPES]
  error.value = ''
  loadApplications()
})

const toggleScope = (s: string) => {
  const i = scopes.value.indexOf(s)
  if (i >= 0) scopes.value.splice(i, 1)
  else scopes.value.push(s)
}

const handleSubmit = async () => {
  error.value = ''
  if (!name.value.trim()) {
    error.value = '请填写密钥名称'
    return
  }
  if (scopes.value.length === 0) {
    error.value = '请至少选择一个 scope'
    return
  }
  isLoading.value = true
  try {
    const body: Record<string, unknown> = {
      name: name.value.trim(),
      test: test.value,
      scopes: scopes.value
    }
    const res = await api.post<DevKeyMinted>(
      `/dev/apps/${props.clientId}/keys`,
      body
    )
    if (res.code === 0 && res.data) {
      open.value = false
      emit('minted', res.data)
    } else {
      error.value = res.message || '生成失败'
    }
  } finally {
    isLoading.value = false
  }
}
</script>

<template>
  <KunModal v-model="open" size="md" aria-label="生成新密钥">
    <div class="space-y-4">
      <h2 class="text-xl font-bold text-foreground">生成新密钥</h2>

      <KunInput
        v-model="name"
        label="密钥名称"
        placeholder="例如：生产环境 / CI 流水线"
        required
      />

      <div>
        <span class="mb-1 block text-sm font-medium text-default-500">
          Scope（权限范围）
          <span class="text-xs text-default-400">— 默认全选公开只读 scope</span>
        </span>
        <div class="flex flex-wrap gap-2">
          <KunCheckBox
            v-for="s in DEV_MINTABLE_SCOPES"
            :key="s"
            :model-value="scopes.includes(s)"
            :label="s"
            color="primary"
            class-name="rounded-lg border border-default-200 bg-content1 px-3 py-1.5 hover:border-primary"
            @update:model-value="toggleScope(s)"
          />
        </div>

        <div
          v-for="s in DEV_GRANTABLE_SCOPES"
          :key="`grant-${s}`"
          class="mt-2 rounded-lg border border-default-200 bg-content1 px-3 py-2"
        >
          <div class="flex flex-wrap items-center gap-2">
            <KunCheckBox
              :model-value="scopes.includes(s)"
              :label="s"
              color="primary"
              :disabled="!isGranted(s)"
              @update:model-value="toggleScope(s)"
            />
            <KunChip color="warning" variant="flat" size="xs">授权制</KunChip>
            <KunChip
              v-if="applicationFor(s)"
              :color="DEV_SCOPE_APP_STATUS_COLORS[applicationFor(s)!.status]"
              variant="flat"
              size="xs"
            >
              {{ DEV_SCOPE_APP_STATUS_LABELS[applicationFor(s)!.status] }}
            </KunChip>
            <KunButton
              v-if="!isGranted(s) && applicationFor(s)?.status !== 'pending'"
              size="xs"
              variant="flat"
              color="primary"
              class="ml-auto"
              :disabled="scopeApplyDisabled"
              @click="openApply(s)"
            >
              {{ applicationFor(s) ? '重新申请' : '申请授权' }}
            </KunButton>
          </div>
          <p v-if="scopeApplyDisabled" class="mt-1 text-xs text-default-400">
            {{ DEV_DISABLED_HINT }}：暂不接受新的 scope 授权申请。
          </p>
          <p
            v-if="applicationFor(s)?.status === 'declined'"
            class="mt-1 text-xs text-danger"
          >
            未获批准：{{ applicationFor(s)!.decline_reason }}
          </p>
          <p v-else-if="!isGranted(s)" class="mt-1 text-xs text-default-400">
            资讯 API 的权限由平台授予：提交申请说明用途，批准后即可在这里自助勾选。
          </p>
        </div>
      </div>

      <div class="rounded-lg border border-default-200 p-3">
        <KunSwitch v-model="test" label="测试密钥（nmk_test_ 前缀）" />
        <p class="mt-1 text-xs text-default-400">
          — 测试密钥用于开发/联调；正式接入请使用生产密钥（nmk_live_）。
        </p>
      </div>

      <div v-if="error" class="rounded-lg bg-danger-50 p-3 text-sm text-danger">
        {{ error }}
      </div>

      <div class="flex justify-end gap-3">
        <KunButton color="default" variant="flat" @click="open = false">
          取消
        </KunButton>
        <KunButton color="primary" :disabled="isLoading" @click="handleSubmit">
          <KunIcon
            v-if="isLoading"
            name="lucide:loader-circle"
            class="mr-2 size-4 animate-spin"
          />
          生成密钥
        </KunButton>
      </div>
    </div>

    <KeysScopeApplyModal
      v-model:open="applyOpen"
      :scope="applyScope"
      @filed="handleFiled"
    />
  </KunModal>
</template>
