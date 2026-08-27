interface AvatarUploadResponse {
  code: number
  message: string
  data?: { hash: string }
}

export const useAvatarUpload = () => {
  const uploading = ref(false)
  const error = ref('')

  const upload = async (
    blob: Blob,
    endpoint: string
  ): Promise<string | null> => {
    if (uploading.value) return null
    uploading.value = true
    error.value = ''

    try {
      const fd = new FormData()
      fd.append('file', blob, 'avatar.webp')

      const cfg = useRuntimeConfig()
      const cookie = useCookie('access_token')
      const res = await $fetch<AvatarUploadResponse>(
        `${cfg.public.apiBase}${endpoint}`,
        {
          method: 'POST',
          body: fd,
          headers: cookie.value
            ? { Authorization: `Bearer ${cookie.value}` }
            : {},
          credentials: 'include',
        }
      )

      if (res.code === 0 && res.data?.hash) {
        return res.data.hash
      }
      error.value = res.message || '上传失败'
      return null
    } catch (err) {
      const e = err as {
        data?: { message?: string }
        statusMessage?: string
        message?: string
      }
      error.value = e?.data?.message || e?.statusMessage || e?.message || '网络错误'
      return null
    } finally {
      uploading.value = false
    }
  }

  return { uploading, error, upload }
}
