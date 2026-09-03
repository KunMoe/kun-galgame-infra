// 1e6 and not lower: zh-CN compact rounds 999,999 up to "100万", which reads as a
// million the account never made. Six digits still fit the stat tile ungrouped.
const COMPACT_FLOOR = 1_000_000

export const formatCompactNumber = (n: number): string => {
  if (!Number.isFinite(n) || Math.abs(n) < COMPACT_FLOOR) {
    return n.toLocaleString()
  }
  return new Intl.NumberFormat('zh-CN', {
    notation: 'compact',
    maximumFractionDigits: 1
  }).format(n)
}
