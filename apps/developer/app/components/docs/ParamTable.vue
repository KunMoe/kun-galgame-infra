<script setup lang="ts">
import type { DocsParam } from '~~/shared/types/docs'

defineProps<{ params: DocsParam[] }>()

const { t } = useDocsI18n()
const inLabel: Record<string, string> = { path: 'path', query: 'query' }
</script>

<template>
  <div class="border-default-200 overflow-x-auto rounded-xl border">
    <table class="w-full min-w-[36rem] text-sm">
      <thead>
        <tr class="border-default-200 text-default-400 border-b text-left">
          <th class="px-4 py-2 font-medium">参数</th>
          <th class="px-4 py-2 font-medium">类型</th>
          <th class="px-4 py-2 font-medium">位置</th>
          <th class="px-4 py-2 font-medium">说明</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="(p, i) in params"
          :key="`${p.in}-${p.name}`"
          class="border-default-100 border-b align-top"
          :class="i === params.length - 1 && 'border-b-0'"
        >
          <td class="px-4 py-3 whitespace-nowrap">
            <code class="text-foreground font-mono font-medium">{{
              p.name
            }}</code>
            <span
              v-if="p.required"
              class="text-danger-600 ml-1.5 text-[0.625rem] font-semibold tracking-wide uppercase"
            >
              必填
            </span>
          </td>
          <td class="px-4 py-3 whitespace-nowrap">
            <code class="text-default-500 font-mono text-xs">{{ p.type }}</code>
            <code
              v-if="p.format"
              class="text-default-300 ml-1 font-mono text-xs"
            >
              {{ p.format }}
            </code>
          </td>
          <td class="px-4 py-3">
            <code class="text-default-400 font-mono text-xs">
              {{ inLabel[p.in] ?? p.in }}
            </code>
          </td>
          <td class="text-default-500 px-4 py-3">
            <p v-if="p.doc" class="leading-relaxed">{{ t(p.doc) }}</p>
            <div v-if="p.enum" class="mt-1 flex flex-wrap items-center gap-1">
              <code
                v-for="v in p.enum"
                :key="v"
                class="bg-default-100 text-default-600 rounded px-1.5 py-px font-mono text-xs"
              >
                {{ v }}
              </code>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
