<script setup lang="ts">
const { startLogin, startRegister } = useOAuthLogin()
const { isOpen, redirect } = useLoginModal()

const error = ref('')
const ssoLoading = ref(false)

const reset = () => {
  error.value = ''
  ssoLoading.value = false
}

watch(isOpen, (open) => {
  if (!open) reset()
})

const handleSso = async () => {
  if (ssoLoading.value) return
  ssoLoading.value = true
  error.value = ''
  try {
    await startLogin(redirect.value ?? undefined)
  } catch {
    ssoLoading.value = false
    error.value = '无法发起登录，请重试'
  }
}

const handleRegister = async () => {
  if (ssoLoading.value) return
  ssoLoading.value = true
  try {
    await startRegister(redirect.value ?? undefined)
  } catch {
    ssoLoading.value = false
    error.value = '无法发起注册，请重试'
  }
}
</script>

<template>
  <KunModal v-model="isOpen" size="md" aria-label="登录开发者平台">
    <div class="space-y-6">
      <div class="space-y-1 text-center">
        <div
          class="bg-primary mx-auto flex size-11 items-center justify-center rounded-xl text-white"
        >
          <KunIcon name="lucide:boxes" class="size-6" />
        </div>
        <h2 class="text-foreground pt-2 text-xl font-bold">登录开发者平台</h2>
        <p class="text-default-500 text-sm">使用你的 NextMoe（未萌）账号登录</p>
      </div>

      <KunButton
        color="primary"
        size="lg"
        class="w-full"
        :disabled="ssoLoading"
        @click="handleSso"
      >
        <KunIcon
          v-if="ssoLoading"
          name="lucide:loader-circle"
          class="mr-2 size-4 animate-spin"
        />
        <KunIcon v-else name="lucide:log-in" class="mr-2 size-4" />
        使用 NextMoe 账号登录
      </KunButton>

      <div v-if="error" class="bg-danger-50 text-danger rounded-lg p-3 text-sm">
        {{ error }}
      </div>

      <p class="text-default-500 text-center text-sm">
        还没有账号？
        <button
          type="button"
          class="text-primary hover:underline"
          :disabled="ssoLoading"
          @click="handleRegister"
        >
          注册 NextMoe 账号
        </button>
      </p>
    </div>
  </KunModal>
</template>
