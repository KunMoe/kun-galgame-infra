<script setup lang="ts">
const auth = useAuth()
const { startLogin, startRegister } = useOAuthLogin()
const { isOpen, redirect, close } = useLoginModal()

const account = ref('')
const password = ref('')
const error = ref('')
const isLoading = ref(false) // password submit in flight
const ssoLoading = ref(false) // SSO / register redirect being prepared

const reset = () => {
  account.value = ''
  password.value = ''
  error.value = ''
  isLoading.value = false
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
    await startLogin(redirect.value ?? undefined) // navigates away
  } catch {
    ssoLoading.value = false
    error.value = '无法发起登录，请重试'
  }
}

const handleRegister = async () => {
  if (ssoLoading.value) return
  ssoLoading.value = true
  try {
    await startRegister(redirect.value ?? undefined) // navigates away
  } catch {
    ssoLoading.value = false
    error.value = '无法发起注册，请重试'
  }
}

const handleSubmit = async () => {
  if (isLoading.value) return
  error.value = ''
  isLoading.value = true
  try {
    const response = await auth.login(account.value, password.value)
    if (response.code !== 0) {
      error.value = response.message || '登录失败'
      return
    }
    const to = redirect.value || '/dashboard'
    close()
    await navigateTo(to)
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : '登录失败'
  } finally {
    isLoading.value = false
  }
}
</script>

<template>
  <KunModal v-model="isOpen" size="md" aria-label="登录开发者平台">
    <div class="space-y-6">
      <div class="space-y-1 text-center">
        <div
          class="mx-auto flex size-11 items-center justify-center rounded-xl bg-primary text-white"
        >
          <KunIcon name="lucide:boxes" class="size-6" />
        </div>
        <h2 class="pt-2 text-xl font-bold text-foreground">登录开发者平台</h2>
        <p class="text-sm text-default-500">
          使用你的 NextMoe（未萌）账号登录
        </p>
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

      <div v-if="error" class="rounded-lg bg-danger-50 p-3 text-sm text-danger">
        {{ error }}
      </div>

      <div class="flex items-center gap-3 text-xs text-default-400">
        <span class="h-px flex-1 bg-default-200" />
        或使用密码登录
        <span class="h-px flex-1 bg-default-200" />
      </div>

      <form class="space-y-4" @submit.prevent="handleSubmit">
        <KunInput
          v-model="account"
          label="账号"
          type="text"
          placeholder="邮箱或用户名"
          required
        />
        <KunInput
          v-model="password"
          label="密码"
          type="password"
          placeholder="请输入密码"
          required
        />
        <KunButton
          type="submit"
          variant="flat"
          class="w-full"
          :disabled="isLoading"
        >
          <KunIcon
            v-if="isLoading"
            name="lucide:loader-circle"
            class="mr-2 size-4 animate-spin"
          />
          {{ isLoading ? '登录中…' : '密码登录' }}
        </KunButton>
      </form>

      <p class="text-center text-sm text-default-500">
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
