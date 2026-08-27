<script setup lang="ts">
const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{
  user: { uuid: string; name: string } | null
}>()
const emit = defineEmits<{ success: [] }>()

const api = useApi()

const name = ref('')
const email = ref('')
const bio = ref('')
const initial = ref<{ name: string; email: string; bio: string } | null>(null)

const loading = ref(false)
const loadFailed = ref(false)
const error = ref('')
const submitting = ref(false)

watch(
  [open, () => props.user?.uuid],
  async ([v]) => {
    if (!v || !props.user) return
    loading.value = true
    loadFailed.value = false
    error.value = ''
    initial.value = null
    name.value = ''
    email.value = ''
    bio.value = ''

    const res = await api.get<{
      name: string
      email: string
      bio: string
    }>(`/admin/users/${props.user.uuid}`)
    if (res.code === 0 && res.data) {
      name.value = res.data.name ?? ''
      email.value = res.data.email ?? ''
      bio.value = res.data.bio ?? ''
      initial.value = {
        name: name.value,
        email: email.value,
        bio: bio.value
      }
    } else {
      error.value = res.message || '加载用户信息失败'
      loadFailed.value = true
    }
    loading.value = false
  },
  { immediate: true }
)

const dirtyPatch = computed(() => {
  if (!initial.value) return null
  const patch: { name?: string; email?: string; bio?: string } = {}
  if (name.value !== initial.value.name) patch.name = name.value.trim()
  if (email.value !== initial.value.email) patch.email = email.value.trim()
  if (bio.value !== initial.value.bio) patch.bio = bio.value
  return Object.keys(patch).length ? patch : null
})

const handleSubmit = async () => {
  if (submitting.value || !props.user || loadFailed.value) return
  error.value = ''

  const trimmedName = name.value.trim()
  if (trimmedName.length < 2 || trimmedName.length > 17) {
    error.value = '用户名长度需为 2–17 个字符'
    return
  }
  if (bio.value.length > 107) {
    error.value = '个人简介不能超过 107 个字符'
    return
  }
  if (!email.value.trim() || !email.value.includes('@')) {
    error.value = '请填写有效的邮箱地址'
    return
  }
  const patch = dirtyPatch.value
  if (!patch) {
    error.value = '没有任何改动'
    return
  }

  submitting.value = true
  try {
    const res = await api.patch(`/admin/users/${props.user.uuid}`, patch)
    if (res.code === 0) {
      useKunMessage('资料已更新', 'success')
      emit('success')
      open.value = false
    } else {
      error.value = res.message || '更新失败'
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <KunModal v-model="open" aria-label="编辑用户资料">
    <div class="space-y-4">
      <div>
        <h2 class="text-xl font-bold text-foreground">编辑用户资料</h2>
        <p class="text-default-500 mt-1 text-sm">
          用户
          <span class="text-foreground font-semibold">{{ user?.name }}</span>
        </p>
      </div>

      <div v-if="loading" class="flex justify-center py-8">
        <KunIcon
          name="lucide:loader-circle"
          class="text-primary size-6 animate-spin"
        />
      </div>

      <form v-else class="space-y-4" @submit.prevent="handleSubmit">
        <KunInput
          v-model="name"
          label="用户名"
          placeholder="2–17 个字符"
          autocomplete="off"
        />

        <div class="space-y-1">
          <KunInput
            v-model="email"
            type="email"
            label="邮箱"
            autocomplete="off"
          />
          <p class="text-warning-500 text-xs">
            管理员改邮箱不会发送验证码，保存后立即生效。
          </p>
        </div>

        <KunTextarea
          v-model="bio"
          label="个人简介"
          placeholder="一句话介绍（≤107 字）"
          :rows="3"
        />

        <p
          v-if="error"
          class="bg-danger-50 text-danger rounded-lg p-3 text-sm"
        >
          {{ error }}
        </p>

        <div class="flex justify-end gap-3">
          <KunButton
            color="default"
            variant="flat"
            type="button"
            :disabled="submitting"
            @click="open = false"
          >
            取消
          </KunButton>
          <KunButton
            color="primary"
            type="submit"
            :disabled="submitting || loadFailed"
          >
            <KunIcon
              v-if="submitting"
              name="lucide:loader-circle"
              class="mr-2 size-4 animate-spin"
            />
            {{ submitting ? '保存中...' : '保存' }}
          </KunButton>
        </div>
      </form>
    </div>
  </KunModal>
</template>
