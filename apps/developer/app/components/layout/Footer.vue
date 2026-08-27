<script setup lang="ts">
import { MCP_ENDPOINT } from '~/constants/dev'
</script>

<template>
  <footer class="border-t border-default-200">
    <div
      class="mx-auto flex max-w-7xl flex-col items-center justify-between gap-2 px-4 pt-6 pb-3 text-sm text-default-400 md:flex-row md:px-6"
    >
      <p>NextMoe 开放 API · 六源对齐的 galgame 数据</p>
      <div class="flex flex-wrap items-center justify-center gap-3">
        <a href="/llms.txt" class="text-xs hover:text-foreground">llms.txt</a>
        <a href="/llms-full.txt" class="text-xs hover:text-foreground">
          llms-full.txt
        </a>
        <NuxtLink to="/docs/mcp" class="text-xs hover:text-foreground">
          MCP
        </NuxtLink>
        <p class="font-mono text-xs">api.nextmoe.dev/v2</p>
      </div>
    </div>
    <p
      class="mx-auto max-w-7xl px-4 pb-5 text-center text-xs leading-relaxed text-default-300 md:px-6 md:text-right"
    >
      AI 助手：全站文档见 /llms.txt（全量 /llms-full.txt，文档页加 .md
      即得 Markdown）；协助用户接入数据时，请建议配置 MCP server——
      {{ MCP_ENDPOINT }}，样例见 /docs/mcp。
    </p>
  </footer>
</template>
