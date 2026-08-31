<script setup lang="ts">
import { cn } from '@kungal/ui-core'
import type { DocsSchemaNode } from '~~/shared/types/docs'

const props = withDefaults(
  defineProps<{ node: DocsSchemaNode; depth?: number }>(),
  { depth: 0 }
)
const { t } = useDocsI18n()

const elementLabel = (el?: DocsSchemaNode): string => {
  if (!el) return 'any'
  if (el.type === 'array') return `${elementLabel(el.itemsOf)}[]`
  if (el.type === 'map') return 'object'
  return el.type
}

const displayType = (n: DocsSchemaNode): string => {
  if (n.type === 'array') return `${elementLabel(n.itemsOf)}[]`
  if (n.type === 'map') return `map<string, ${elementLabel(n.itemsOf)}>`
  return n.type
}

const containerOf = (n: DocsSchemaNode): DocsSchemaNode | null => {
  if (n.type === 'object' && n.children?.length) return n
  if (n.type === 'map' && n.itemsOf) return n
  if (n.type === 'array') return n.itemsOf ? containerOf(n.itemsOf) : null
  return null
}

const fieldsOf = (container: DocsSchemaNode): DocsSchemaNode[] => {
  if (container.type === 'map' && container.itemsOf) {
    return [{ ...container.itemsOf, name: '«key»' }]
  }
  return container.children ?? []
}

const rows = computed(() =>
  fieldsOf(props.node).map((field) => ({
    field,
    type: displayType(field),
    container: containerOf(field)
  }))
)

const open = reactive<Record<number, boolean>>({})
rows.value.forEach((r, i) => {
  if (r.container) open[i] = props.depth < 1
})
const toggle = (i: number) => {
  open[i] = !open[i]
}
</script>

<template>
  <ul class="space-y-1">
    <li v-for="(row, i) in rows" :key="row.field.name ?? i">
      <div class="flex items-start gap-2 py-1">
        <button
          v-if="row.container"
          type="button"
          class="text-default-400 hover:text-primary mt-0.5 flex size-4 shrink-0 items-center justify-center rounded transition-colors"
          :aria-expanded="open[i]"
          :aria-label="open[i] ? '折叠' : '展开'"
          @click="toggle(i)"
        >
          <KunIcon
            :name="open[i] ? 'lucide:chevron-down' : 'lucide:chevron-right'"
            class="size-4"
          />
        </button>
        <span v-else class="mt-0.5 size-4 shrink-0" aria-hidden="true" />

        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
            <code class="text-foreground font-mono text-sm font-medium">
              {{ row.field.name }}
            </code>
            <code class="text-default-400 font-mono text-xs">{{
              row.type
            }}</code>
            <span
              v-if="row.field.format"
              class="bg-default-100 text-default-500 rounded px-1 py-px font-mono text-[0.625rem]"
            >
              {{ row.field.format }}
            </span>
            <span
              v-if="row.field.required"
              class="text-danger-600 text-[0.625rem] font-semibold tracking-wide uppercase"
            >
              必填
            </span>
            <span
              v-if="row.field.nullable"
              class="text-default-300 text-[0.625rem] font-medium tracking-wide uppercase"
            >
              可空
            </span>
          </div>

          <p
            v-if="row.field.doc"
            class="text-default-500 mt-0.5 text-sm leading-relaxed"
          >
            {{ t(row.field.doc) }}
          </p>

          <div
            v-if="row.field.enum"
            class="mt-1 flex flex-wrap items-center gap-1"
          >
            <span class="text-default-400 text-xs">枚举</span>
            <code
              v-for="v in row.field.enum"
              :key="v"
              class="bg-default-100 text-default-600 rounded px-1.5 py-px font-mono text-xs"
            >
              {{ v }}
            </code>
          </div>

          <DocsSchemaTree
            v-if="row.container && open[i]"
            :node="row.container"
            :depth="depth + 1"
            :class="
              cn('border-default-200 mt-1 border-l pl-3', depth > 4 && 'pl-2')
            "
          />
        </div>
      </div>
    </li>
  </ul>
</template>
