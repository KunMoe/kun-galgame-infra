<script setup lang="ts">
import type { DevApp } from '~~/shared/types/dev'

const props = defineProps<{ app: DevApp }>()
const emit = defineEmits<{ updated: [DevApp] }>()

const open = defineModel<boolean>('open', { required: true })
const api = useApi()

const name = ref(props.app.name)
const description = ref(props.app.description)
const error = ref('')
const isLoading = ref(false)

watch(open, (val) => {
  if (!val) return
  name.value = props.app.name
  description.value = props.app.description
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
    const body: Record<string, unknown> = {
      name: name.value.trim(),
      description: description.value.trim()
    }
    const res = await api.patch<DevApp>(
      `/dev/apps/${props.app.client_id}`,
      body
    )
    if (res.code === 0 && res.data) {
      useKunMessage('已保存', 'success')
      open.value = false
      emit('updated', res.data)
    } else {
      error.value = res.message || '保存失败'
    }
  } finally {
    isLoading.value = false
  }
}
</script>

<template>
  <KunModal v-model="open" size="md" aria-label="编辑应用">
    <div class="space-y-4">
      <h2 class="text-foreground text-xl font-bold">编辑应用</h2>

      <KunInput v-model="name" label="应用名称" required />
      <KunInput
        v-model="description"
        label="应用描述（可选）"
        placeholder="一句话描述用途,最多 100 字"
      />

      <div v-if="error" class="bg-danger-50 text-danger rounded-lg p-3 text-sm">
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
          保存
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
