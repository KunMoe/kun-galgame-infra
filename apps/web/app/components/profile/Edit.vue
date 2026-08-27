<script setup lang="ts">
import { resolveAvatarUrl } from '~~/shared/utils/resolveImage'

const auth = useAuth()
const user = auth.user
const userStore = useUserStore()

const name = ref('')
const bio = ref('')
const error = ref('')
const success = ref('')
const isLoading = ref(false)

const avatarFile = ref<Blob | null>(null)
const avatarCrop = ref<{ reset: () => void } | null>(null)
const {
  uploading: avatarUploading,
  error: avatarError,
  upload: uploadAvatar,
} = useAvatarUpload()

const cdnBase = useRuntimeConfig().public.imageCdnBase as string
const avatarSrc = computed(() =>
  resolveAvatarUrl(user.value ?? null, { cdnBase, variant: '256' }, '')
)

watchEffect(() => {
  if (!user.value) return
  name.value = user.value.name ?? ''
  bio.value = user.value.bio ?? ''
})

const dirty = computed(
  () =>
    !!user.value &&
    (name.value !== (user.value.name ?? '') ||
      bio.value !== (user.value.bio ?? ''))
)

const handleSubmit = async () => {
  if (isLoading.value || avatarUploading.value) return
  error.value = ''
  success.value = ''

  if (name.value.trim().length < 2 || name.value.trim().length > 17) {
    error.value = '用户名长度需为 2–17 个字符'
    return
  }
  if (bio.value.length > 107) {
    error.value = '个人简介不能超过 107 个字符'
    return
  }
  if (!dirty.value) {
    error.value = '没有任何改动'
    return
  }

  const payload: { name?: string; bio?: string } = {}
  if (name.value !== (user.value?.name ?? '')) payload.name = name.value.trim()
  if (bio.value !== (user.value?.bio ?? '')) payload.bio = bio.value

  isLoading.value = true
  try {
    const response = await auth.updateProfile(payload)
    if (response.code === 0) {
      success.value = '资料已更新'
    } else {
      error.value = response.message || '更新失败'
    }
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : '更新失败'
  } finally {
    isLoading.value = false
  }
}

const handleAvatarUpload = async () => {
  if (!avatarFile.value || avatarUploading.value || isLoading.value) return
  const hash = await uploadAvatar(avatarFile.value, '/auth/me/avatar')
  if (hash && userStore.user) {
    userStore.setUser({ ...userStore.user, avatar_image_hash: hash })
    useKunMessage('头像已更新', 'success')
    avatarCrop.value?.reset()
    avatarFile.value = null
  }
}
</script>

<template>
  <KunCard class="p-6">
    <h3 class="mb-4 text-lg font-semibold text-foreground">
      <KunIcon name="lucide:user-pen" class="mr-2 inline size-5" />
      编辑资料
    </h3>

    <form class="space-y-4" @submit.prevent="handleSubmit">
      <div class="space-y-2 text-center">
        <KunAvatar
          :user="{ id: 0, name: name || '用户', avatar: avatarSrc }"
          size="lg"
          :is-navigation="false"
        />
        <CommonAvatarCrop
          ref="avatarCrop"
          v-model:file="avatarFile"
          :disabled="avatarUploading || isLoading"
        />
        <KunButton
          type="button"
          variant="flat"
          color="primary"
          size="sm"
          :disabled="!avatarFile || avatarUploading || isLoading"
          @click="handleAvatarUpload"
        >
          <KunIcon
            v-if="avatarUploading"
            name="lucide:loader-circle"
            class="mr-1 size-4 animate-spin"
          />
          上传头像
        </KunButton>
        <div v-if="avatarError" class="text-danger-600 text-sm">
          {{ avatarError }}
        </div>
      </div>

      <KunInput
        v-model="name"
        label="用户名"
        placeholder="2–17 个字符，全站唯一"
        required
        autocomplete="username"
      />

      <KunTextarea
        v-model="bio"
        label="个人简介"
        placeholder="一句话介绍自己（≤107 字）"
        :rows="3"
      />

      <div v-if="error" class="rounded-lg bg-danger-50 p-3 text-sm text-danger">
        {{ error }}
      </div>

      <div v-if="success" class="rounded-lg bg-success-50 p-3 text-sm text-success">
        {{ success }}
      </div>

      <KunButton
        type="submit"
        color="primary"
        class="w-full"
        :disabled="isLoading || !dirty || avatarUploading"
      >
        <KunIcon
          v-if="isLoading"
          name="lucide:loader-circle"
          class="mr-2 size-4 animate-spin"
        />
        {{ isLoading ? '保存中...' : '保存修改' }}
      </KunButton>
    </form>
  </KunCard>
</template>
