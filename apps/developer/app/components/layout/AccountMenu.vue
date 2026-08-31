<script setup lang="ts">
import { ACCOUNT_NAV } from '~/constants/nav'

const auth = useAuth()

const avatarUser = computed(() => {
  const user = auth.user.value
  if (!user) return null
  return { id: 0, name: user.name, avatar: user.avatar || '' }
})

const items = [
  ...ACCOUNT_NAV.map((item) => ({
    key: item.to,
    label: item.label,
    icon: item.icon,
    href: item.to
  })),
  {
    key: 'logout',
    label: '退出登录',
    icon: 'lucide:log-out',
    color: 'danger' as const
  }
]

const onSelect = async (item: { key: string }) => {
  if (item.key === 'logout') await auth.logout()
}
</script>

<template>
  <KunDropdown
    :items="items"
    position="bottom-end"
    :min-width="200"
    @select="onSelect"
  >
    <template #trigger>
      <KunAvatar
        :user="avatarUser"
        size="sm"
        :is-navigation="false"
        disable-floating
      />
    </template>
  </KunDropdown>
</template>
