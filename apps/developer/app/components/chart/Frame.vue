<script setup lang="ts">
import type { EChartsOption } from 'echarts'

const props = withDefaults(
  defineProps<{
    option: EChartsOption
    height?: number
    /** Rendered instead of the plot when there is nothing to draw. */
    empty?: boolean
    emptyText?: string
  }>(),
  { height: 260, empty: false, emptyText: '暂无数据' }
)

const emit = defineEmits<{ select: [dataIndex: number] }>()

const style = computed(() => ({ height: `${props.height}px` }))
</script>

<template>
  <div v-if="empty" class="flex items-center justify-center" :style="style">
    <p class="text-default-400 text-sm">{{ emptyText }}</p>
  </div>
  <ClientOnly v-else>
    <VChart
      :option="option"
      :style="style"
      autoresize
      @click="emit('select', ($event as { dataIndex: number }).dataIndex)"
    />
    <template #fallback>
      <div class="bg-default-100 animate-pulse rounded-lg" :style="style" />
    </template>
  </ClientOnly>
</template>
