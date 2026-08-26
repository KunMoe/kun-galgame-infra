<script setup lang="ts">

interface Props {
  open: boolean
  user: { uuid: string; name: string } | null
}
interface Emits {
  (e: 'update:open', v: boolean): void
  (e: 'success', hash: string): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const file = ref<Blob | null>(null)
const uploading = ref(false)
const errorMsg = ref('')
const uploadKey = ref(0)

const onCropped = (blob: Blob) => {
  errorMsg.value = ''
  file.value = blob
}

const close = () => {
  if (uploading.value) return
  emit('update:open', false)
  setTimeout(() => {
    file.value = null
    errorMsg.value = ''
    uploadKey.value++
  }, 200)
}

const submit = async () => {
  if (!file.value || !props.user || uploading.value) return
  uploading.value = true
  errorMsg.value = ''

  try {
    const fd = new FormData()
    fd.append('file', file.value, 'avatar.webp')

    const cfg = useRuntimeConfig()
    const cookie = useCookie('access_token')
    const res = await $fetch<{
      code: number
      message: string
      data?: { hash: string }
    }>(`${cfg.public.apiBase}/admin/users/${props.user.uuid}/avatar`, {
      method: 'POST',
      body: fd,
      headers: cookie.value ? { Authorization: `Bearer ${cookie.value}` } : {},
      credentials: 'include'
    })

    if (res.code === 0 && res.data?.hash) {
      emit('success', res.data.hash)
      close()
    } else {
      errorMsg.value = res.message || '上传失败'
    }
  } catch (err) {
    const e = err as { data?: { message?: string }; statusMessage?: string; message?: string }
    errorMsg.value = e?.data?.message || e?.statusMessage || e?.message || '网络错误'
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <KunModal :model-value="open" @update:model-value="close" aria-label="上传头像">
    <div class="space-y-4">
      <h2 class="text-foreground text-lg font-semibold">
        上传头像 — {{ user?.name }}
      </h2>

      <div class="space-y-3">
        <p class="text-default-500 text-sm">
          会推送到 image_service，自动生成 <code>_256</code> / <code>_100</code>
          变体并写入 <code>avatar_image_hash</code>。原 <code>avatar</code> URL
          保留作回退（老用户头像不会被覆盖）。
        </p>

        <KunUpload
          :key="uploadKey"
          :size="512"
          :aspect="1"
          description="点击或拖拽选择图片，可裁剪为正方形"
          class-name="mx-auto w-48"
          @set-image="onCropped"
        />

        <div v-if="errorMsg" class="text-danger-600 text-sm">
          {{ errorMsg }}
        </div>
      </div>

      <div class="flex justify-end gap-2 pt-2">
        <KunButton variant="flat" :disabled="uploading" @click="close">
          取消
        </KunButton>
        <KunButton
          color="primary"
          :disabled="!file || uploading"
          @click="submit"
        >
          <KunIcon
            v-if="uploading"
            name="lucide:loader-circle"
            class="mr-1 size-4 animate-spin"
          />
          上传
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
