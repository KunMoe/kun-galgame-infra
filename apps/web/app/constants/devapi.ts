
export const DEV_TIERS = ['free', 'trusted', 'internal'] as const
export type DevTier = (typeof DEV_TIERS)[number]

export const DEV_TIER_OPTIONS: { value: DevTier; label: string }[] = [
  { value: 'free', label: 'Free（免费）' },
  { value: 'trusted', label: 'Trusted（信任）' },
  { value: 'internal', label: 'Internal（内部，不限流）' },
]

export const DEV_TIER_COLORS: Record<string, 'default' | 'primary' | 'success'> = {
  free: 'default',
  trusted: 'primary',
  internal: 'success',
}

export const DEV_TIER_LIMITS: Record<
  string,
  { rate: number; quota: number; unlimited: boolean }
> = {
  free: { rate: 60, quota: 50_000, unlimited: false },
  trusted: { rate: 600, quota: 1_000_000, unlimited: false },
  internal: { rate: 0, quota: 0, unlimited: true },
}

export const DEV_MINTABLE_SCOPES = ['catalog:read'] as const

// Mirrors devapi's maxAppReviewNoteLen (counted in runes server-side).
export const DEV_APP_REVIEW_NOTE_MAX = 2000

export const DEV_APP_STATUS_TABS = [
  { id: 'enabled', label: '已启用', icon: 'lucide:check' },
  { id: 'pending', label: '待审核', icon: 'lucide:clock' },
  { id: 'declined', label: '已拒绝', icon: 'lucide:x' },
  { id: 'disabled', label: '已停用', icon: 'lucide:power-off' },
  { id: 'all', label: '全部', icon: 'lucide:list' },
]

export const DEV_APP_REVIEW_LABELS: Record<string, string> = {
  approved: '已通过',
  pending: '待审核',
  declined: '已拒绝',
}

export const DEV_APP_REVIEW_COLORS: Record<
  string,
  'success' | 'warning' | 'danger'
> = {
  approved: 'success',
  pending: 'warning',
  declined: 'danger',
}

// Rows written before the approval flow existed carry an empty status and are
// live applications; showing them as "unreviewed" would be a lie.
export const devAppReviewLabel = (status: string): string =>
  DEV_APP_REVIEW_LABELS[status] ?? DEV_APP_REVIEW_LABELS.approved!

export const devAppReviewColor = (
  status: string
): 'success' | 'warning' | 'danger' =>
  DEV_APP_REVIEW_COLORS[status] ?? 'success'

export const DEV_POLICY_MODE_LABELS: Record<string, string> = {
  self_service: '自助',
  approval: '需审批',
  disabled: '已关闭',
}

// Written out rather than composed from the mode name: Tailwind only emits the
// class names it can see as literals in the source.
export const DEV_POLICY_MODE_ACTIVE_CLASS: Record<string, string> = {
  self_service: 'border-success bg-success-50',
  approval: 'border-warning bg-warning-50',
  disabled: 'border-danger bg-danger-50',
}

export const DEV_POLICY_MODE_ICON_CLASS: Record<string, string> = {
  self_service: 'text-success',
  approval: 'text-warning',
  disabled: 'text-danger',
}

export const DEV_POLICY_MODE_HINTS: Record<string, string> = {
  self_service: '用户可以直接完成，无需平台介入',
  approval: '用户提交后进入待审核，由管理台放行',
  disabled: '该功能对所有用户关闭',
}

export const DEV_KEY_STATE_TABS = [
  { id: 'all', label: '全部', icon: 'lucide:list' },
  { id: 'active', label: '活跃', icon: 'lucide:check' },
  { id: 'expired', label: '已过期', icon: 'lucide:clock' },
  { id: 'revoked', label: '已吊销', icon: 'lucide:ban' },
]

export const DEV_KEY_STATE_LABELS: Record<string, string> = {
  active: '活跃',
  expired: '已过期',
  revoked: '已吊销',
}

export const DEV_KEY_STATE_COLORS: Record<
  string,
  'success' | 'default' | 'danger'
> = {
  active: 'success',
  expired: 'default',
  revoked: 'danger',
}

export const DEV_KEY_PAGE_SIZE = 50

export const devTierLimitHint = (tier: string): string => {
  const l = DEV_TIER_LIMITS[tier]
  if (!l) return ''
  if (l.unlimited) return '不限流 / 不限配额'
  return `${l.rate} 次/分 · ${l.quota.toLocaleString()} 次/日`
}
