<script setup lang="ts">
const props = withDefaults(
  defineProps<{ values: number[]; width?: number; height?: number }>(),
  { width: 96, height: 24 }
)

// Hand-drawn rather than a fifth ECharts instance: a stat tile's spark is four
// points of context, and each mounted chart brings its own canvas and resize
// observer.
const path = computed(() => {
  const v = props.values
  if (v.length < 2) return ''
  const max = Math.max(...v)
  const min = Math.min(...v)
  const span = max - min || 1
  const step = props.width / (v.length - 1)
  return v
    .map((n, i) => {
      const x = (i * step).toFixed(2)
      const y = (props.height - ((n - min) / span) * props.height).toFixed(2)
      return `${i === 0 ? 'M' : 'L'}${x} ${y}`
    })
    .join(' ')
})
</script>

<template>
  <svg
    v-if="path"
    :viewBox="`0 -1 ${width} ${height + 2}`"
    :width="width"
    :height="height + 2"
    fill="none"
    aria-hidden="true"
    class="text-primary overflow-visible"
  >
    <path
      :d="path"
      stroke="currentColor"
      stroke-width="1.5"
      stroke-linecap="round"
      stroke-linejoin="round"
      vector-effect="non-scaling-stroke"
    />
  </svg>
</template>
