<script setup lang="ts">
import { DEV_SCOPE_APP_MESSAGE_MAX } from '~/constants/dev'
import type { DevScopeApplication } from '~~/shared/types/dev'

const props = defineProps<{ scope: string }>()
const emit = defineEmits<{ filed: [DevScopeApplication] }>()

const open = defineModel<boolean>('open', { required: true })
const api = useApi()

const message = ref('')
const error = ref('')
const isLoading = ref(false)

watch(open, (val) => {
  if (!val) return
  message.value = ''
  error.value = ''
})

const handleSubmit = async () => {
  error.value = ''
  if (!message.value.trim()) {
    error.value = '请填写用途说明'
    return
  }
  isLoading.value = true
  try {
    const res = await api.post<DevScopeApplication>('/dev/scope-applications', {
      scope: props.scope,
      message: message.value.trim()
    })
    if (res.code === 0 && res.data) {
      emit('filed', res.data)
    } else {
      error.value = res.message || '提交失败'
    }
  } finally {
    isLoading.value = false
  }
}
</script>

<template>
  <KunModal v-model="open" size="md" :aria-label="`申请 ${scope}`">
    <div class="space-y-4">
      <div>
        <h2 class="text-xl font-bold text-foreground">申请 {{ scope }}</h2>
        <p class="mt-1 text-sm text-default-500">
          它只解锁 v1 的 /v1/news；同一份资讯在 /v2/news 上无需任何凭据，多数接入不必申请。
        </p>
        <p class="mt-1 text-sm text-default-500">
          v1 面的资讯来自合作媒体，它们授权给 NextMoe 的是一份索引，转授给谁由平台逐个决定。
          说清楚你要拿它做什么，我们审阅后批准或拒绝；批准后你就能自己给密钥勾上这一项。
        </p>
      </div>

      <KunTextarea
        v-model="message"
        label="用途说明"
        placeholder="例如：为一个 Galgame 发售日历站做资讯侧栏，每条都会展示来源与回源链接。"
        :rows="5"
        :maxlength="DEV_SCOPE_APP_MESSAGE_MAX"
        show-char-count
      />

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
          提交申请
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
