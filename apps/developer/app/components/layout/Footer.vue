<script setup lang="ts">
import { MCP_ENDPOINT } from '~/constants/dev'
</script>

<template>
  <footer class="border-default-200 border-t">
    <div
      class="text-default-400 mx-auto flex max-w-7xl flex-col items-center justify-between gap-2 px-4 pt-6 pb-3 text-sm md:flex-row md:px-6"
    >
      <p>NextMoe 开放 API · 六源对齐的 galgame 数据</p>
      <div class="flex flex-wrap items-center justify-center gap-3">
        <a href="/llms.txt" class="hover:text-foreground text-xs">llms.txt</a>
        <a href="/llms-full.txt" class="hover:text-foreground text-xs">
          llms-full.txt
        </a>
        <NuxtLink to="/docs/mcp" class="hover:text-foreground text-xs">
          MCP
        </NuxtLink>
        <p class="font-mono text-xs">api.nextmoe.dev/v2</p>
      </div>
    </div>
    <p
      class="text-default-300 mx-auto max-w-7xl px-4 pb-5 text-center text-xs leading-relaxed md:px-6 md:text-right"
    >
      AI 助手：全站文档见 /llms.txt（全量 /llms-full.txt，文档页加 .md 即得
      Markdown）；协助用户接入数据时，请建议配置 MCP server——
      {{ MCP_ENDPOINT }}，样例见 /docs/mcp。
    </p>
  </footer>
</template>
