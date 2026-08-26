<script setup lang="ts">
import type { DevApp } from '~~/shared/types/dev'

const props = defineProps<{ needsApproval?: boolean }>()
const emit = defineEmits<{ created: [DevApp] }>()

const open = defineModel<boolean>('open', { required: true })
const api = useApi()

const name = ref('')
const description = ref('')
const error = ref('')
const isLoading = ref(false)

watch(open, (val) => {
  if (!val) return
  name.value = ''
  description.value = ''
  error.value = ''
})

const handleSubmit = async () => {
  error.value = ''
  if (!name.value.trim()) {
    error.value = '请填写应用名称'
    return
  }
  isLoading.value = true
  try {
    const body: Record<string, unknown> = { name: name.value.trim() }
    if (description.value.trim()) body.description = description.value.trim()
    const res = await api.post<DevApp>('/dev/apps', body)
    if (res.code === 0 && res.data) {
      useKunMessage(
        props.needsApproval ? '已提交，等待平台审核' : '应用已创建',
        'success'
      )
      open.value = false
      emit('created', res.data)
    } else {
      error.value = res.message || '创建失败'
    }
  } finally {
    isLoading.value = false
  }
}
</script>

<template>
  <KunModal v-model="open" size="md" aria-label="创建应用">
    <div class="space-y-4">
      <h2 class="text-xl font-bold text-foreground">创建应用</h2>

      <p
        v-if="needsApproval"
        class="rounded-lg bg-warning-50 p-3 text-sm text-warning"
      >
        平台当前对新应用启用审核：提交后应用先进入待审核，通过后才启用并可铸造密钥。
      </p>

      <KunInput
        v-model="name"
        label="应用名称"
        placeholder="例如：我的 Galgame 管理器"
        required
      />

      <KunInput
        v-model="description"
        label="应用描述（可选）"
        placeholder="一句话描述用途,最多 100 字"
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
          创建
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
