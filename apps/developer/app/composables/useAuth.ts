import type { User } from '~~/shared/types/dev'

export const useAuth = () => {
  const api = useApi()
  const userStore = useUserStore()
  const refreshTransient = useRefreshTransient()

  const accessToken = useCookie('access_token', {
    maxAge: 60 * 15, // 15 minutes
    sameSite: 'lax',
    secure: !import.meta.dev
  })

  const authMode = useCookie('auth_mode', {
    maxAge: 60 * 60 * 24 * 90,
    sameSite: 'lax',
    secure: !import.meta.dev
  })

  const setAccessToken = (token: string) => {
    accessToken.value = token
    refreshTransient.value = false
  }

  const clearAuth = () => {
    accessToken.value = null
    authMode.value = null
    userStore.clearUser()
    refreshTransient.value = false // a stale banner must not outlive the session
  }

  const logout = async () => {
    // (/api/v1/auth vs /auth), so a surviving other-mode cookie would silently
    await Promise.allSettled([
      api.post('/auth/logout'),
      $fetch('/auth/logout', { method: 'POST', credentials: 'include' })
    ])
    clearAuth()
    navigateTo('/')
  }

  const refreshAccessToken = async () => {
    const token = await requestTokenRefresh()
    if (typeof token === 'string') {
      setAccessToken(token)
      return true
    }
    return false
  }

  const fetchUser = async () => {
    if (!accessToken.value) {
      const refreshed = await refreshAccessToken()
      if (!refreshed) return null
    }

    const response = await api.get<User>('/auth/me')
    if (response.code === 0 && response.data) {
      userStore.setUser(response.data)
      return response.data
    }
    return null
  }

  return {
    user: computed(() => userStore.user),
    isLoggedIn: computed(() => userStore.isLoggedIn),
    setAccessToken,
    logout,
    fetchUser,
    refreshAccessToken
  }
}
