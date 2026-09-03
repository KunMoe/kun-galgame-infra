import type { EChartsOption } from 'echarts'

/**
 * ECharts colors read from the live KunUI tokens rather than restated as hex.
 *
 * The tokens are OKLCH and their scales invert between light and dark (in dark
 * mode `primary-100` is the DARK end), so a hard-coded hex pair drifts from the
 * palette the rest of the page uses the moment @kungal/ui-tokens ships a new
 * ramp. Reading the custom property keeps one source of truth.
 *
 * The canvas round-trip is the normalisation step: getComputedStyle hands back
 * `oklch(...)`, and while a browser canvas accepts that, zrender parses colors
 * itself for hover/opacity variants and does not understand OKLCH — it silently
 * paints black. Assigning to fillStyle and reading it back returns the browser's
 * own `#rrggbb`, which zrender does understand.
 */
const readTokens = (names: readonly string[]) => {
  const style = getComputedStyle(document.documentElement)
  const canvas = document.createElement('canvas')
  const ctx = canvas.getContext('2d')
  const out: Record<string, string> = {}
  for (const name of names) {
    const raw = style.getPropertyValue(name).trim()
    if (!ctx || !raw) {
      out[name] = raw
      continue
    }
    ctx.fillStyle = '#000'
    ctx.fillStyle = raw
    out[name] = ctx.fillStyle as string
  }
  return out
}

const TOKENS = [
  '--color-primary-500',
  '--color-primary-100',
  '--color-primary-200',
  '--color-primary-300',
  '--color-primary-400',
  '--color-primary-600',
  '--color-warning-500',
  '--color-danger-500',
  '--color-default-200',
  '--color-default-400',
  '--color-default-500',
  '--color-default-600',
  '--color-content1',
  '--color-content3',
  '--color-foreground'
] as const

export interface ChartTheme {
  /** The volume / headline series. */
  accent: string
  /** The de-emphasis series that sits beside `accent` — context, not identity. */
  muted: string
  warning: string
  danger: string
  /** Axis labels and other chart-chrome ink. */
  ink: string
  axis: string
  split: string
  surface: string
  tooltipBg: string
  tooltipText: string
  /** Sequential ramp, low → high, for the calendar heatmap. */
  ramp: string[]
}

const FALLBACK: ChartTheme = {
  accent: '#2881fb',
  muted: '#85858f',
  warning: '#b67700',
  danger: '#ef2663',
  ink: '#6c6c76',
  axis: '#c3c3ce',
  split: '#c3c3ce',
  surface: '#ffffff',
  tooltipBg: '#ffffff',
  tooltipText: '#18181b',
  ramp: ['#e7effe', '#cfdffd', '#a0c6ff', '#6ea4fd', '#2881fb', '#0b60d8']
}

export const useChartTheme = () => {
  const colorMode = useColorMode()
  const theme = ref<ChartTheme>(FALLBACK)

  const refresh = () => {
    if (!import.meta.client) return
    const t = readTokens(TOKENS)
    theme.value = {
      accent: t['--color-primary-500']!,
      muted: t['--color-default-500']!,
      warning: t['--color-warning-500']!,
      danger: t['--color-danger-500']!,
      ink: t['--color-default-600']!,
      axis: t['--color-default-200']!,
      split: t['--color-default-200']!,
      surface: t['--color-content1']!,
      tooltipBg: t['--color-content1']!,
      tooltipText: t['--color-foreground']!,
      ramp: [
        t['--color-primary-100']!,
        t['--color-primary-200']!,
        t['--color-primary-300']!,
        t['--color-primary-400']!,
        t['--color-primary-500']!,
        t['--color-primary-600']!
      ]
    }
  }

  onMounted(() => {
    refresh()
    // The class that carries the dark tokens lands on <html> one tick after the
    // preference changes, so reading synchronously here returns the old ramp.
    watch(
      () => colorMode.value,
      () => nextTick(refresh)
    )
  })

  return theme
}

/**
 * ECharts 6 deprecated `grid.containLabel`. The replacement lays the cartesian
 * rect out first and then shrinks it until the axis labels fit inside
 * `outerBounds`, so the padding belongs on the bounds and the rect itself
 * starts flush.
 */
export const chartGrid = (pad: Partial<ChartPadding> = {}) => {
  const box = { left: 4, right: 8, top: 8, bottom: 0, ...pad }
  return { left: 0, right: 0, top: box.top, bottom: 0, outerBounds: box }
}

export interface ChartPadding {
  left: number
  right: number
  top: number
  bottom: number
}

/** Chrome every chart in the console shares: hairline axes, recessive grid. */
export const chartBase = (t: ChartTheme): EChartsOption => ({
  grid: chartGrid(),
  tooltip: {
    trigger: 'axis',
    backgroundColor: t.tooltipBg,
    borderColor: t.split,
    borderWidth: 1,
    padding: [8, 12],
    textStyle: { color: t.tooltipText, fontSize: 12 },
    axisPointer: { type: 'shadow' }
  },
  textStyle: { fontFamily: 'inherit' }
})

export const categoryAxis = (t: ChartTheme) => ({
  type: 'category' as const,
  axisTick: { show: false },
  axisLine: { lineStyle: { color: t.axis } },
  axisLabel: { color: t.ink, fontSize: 11, hideOverlap: true }
})

export const valueAxis = (t: ChartTheme) => ({
  type: 'value' as const,
  minInterval: 1,
  axisLine: { show: false },
  axisTick: { show: false },
  splitLine: { lineStyle: { color: t.split, type: 'solid' as const } },
  axisLabel: { color: t.ink, fontSize: 11 }
})
