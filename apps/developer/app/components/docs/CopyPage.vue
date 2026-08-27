<script setup lang="ts">
import { SITE_URL } from '~~/shared/brand.mjs'

// Every /docs route has a clean Markdown twin at `<route>.md`, written by
// scripts/gen-llms.mjs into public/. This offers to copy it, copy its link,
// view it, or hand it to ChatGPT / Claude.
const route = useRoute()

const mdPath = computed(() => {
  const p = route.path.replace(/\/+$/, '')
  return p === '' ? '/index.md' : `${p}.md`
})
const mdUrl = computed(() => `${SITE_URL}${mdPath.value}`)
const aiPrompt = () =>
  `阅读 ${mdUrl.value} —— 这是 NextMoe 开放 API 文档页面的 Markdown，我想基于它向你提问。`

const items = [
  { key: 'copy', label: '复制本页 Markdown', icon: 'lucide:clipboard-copy' },
  { key: 'link', label: '复制 Markdown 链接', icon: 'lucide:link' },
  { key: 'view', label: '查看 Markdown', icon: 'lucide:file-text' },
  { key: 'chatgpt', label: '在 ChatGPT 中打开', icon: 'lucide:bot' },
  { key: 'claude', label: '在 Claude 中打开', icon: 'lucide:sparkles' }
]

const openExternal = (url: string) => window.open(url, '_blank', 'noopener')

const copy = async (text: string, ok: string) => {
  try {
    await navigator.clipboard.writeText(text)
    useKunMessage(ok, 'success')
  } catch {
    useKunMessage('复制失败，请重试', 'error')
  }
}

const onSelect = async (item: { key: string }) => {
  switch (item.key) {
    case 'copy': {
      try {
        const md = await $fetch<string>(mdPath.value, { responseType: 'text' })
        await copy(md, '已复制本页 Markdown')
      } catch {
        useKunMessage('获取 Markdown 失败', 'error')
      }
      break
    }
    case 'link':
      await copy(mdUrl.value, '已复制 Markdown 链接')
      break
    case 'view':
      openExternal(mdPath.value)
      break
    case 'chatgpt':
      openExternal(`https://chatgpt.com/?q=${encodeURIComponent(aiPrompt())}`)
      break
    case 'claude':
      openExternal(`https://claude.ai/new?q=${encodeURIComponent(aiPrompt())}`)
      break
  }
}
</script>

<template>
  <KunDropdown :items="items" position="bottom-end" @select="onSelect">
    <template #trigger>
      <KunButton variant="bordered" color="default" size="sm">
        <KunIcon name="lucide:clipboard-copy" class="size-4" />
        <span class="ml-1 hidden sm:inline">复制页面</span>
        <KunIcon name="lucide:chevron-down" class="ml-1 size-4 opacity-60" />
      </KunButton>
    </template>
  </KunDropdown>
</template>
