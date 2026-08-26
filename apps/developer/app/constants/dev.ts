
export const DEV_TIER_COLORS: Record<
  string,
  'default' | 'primary' | 'success'
> = {
  free: 'default',
  trusted: 'primary',
  internal: 'success'
}

export const DEV_TIER_LABELS: Record<string, string> = {
  free: 'Free（免费）',
  trusted: 'Trusted（信任）',
  internal: 'Internal（内部）'
}

export const DEV_MINTABLE_SCOPES = ['catalog:read'] as const

export const DEV_GRANTABLE_SCOPES = ['news:read', 'store:read'] as const

// Mirrors devapi's maxScopeAppMessageLen, which counts runes — so does the
// browser's maxlength (UTF-16 code units for BMP text), and the two agree for
// everything a form like this receives.
export const DEV_SCOPE_APP_MESSAGE_MAX = 2000

export const DEV_SCOPE_APP_STATUS_LABELS: Record<string, string> = {
  pending: '待审核',
  approved: '已批准',
  declined: '已拒绝'
}

export const DEV_SCOPE_APP_STATUS_COLORS: Record<
  string,
  'warning' | 'success' | 'danger'
> = {
  pending: 'warning',
  approved: 'success',
  declined: 'danger'
}

export const DEV_CAP_APP_CREATE = 'app.create'
export const DEV_CAP_APP_MANAGE = 'app.manage'
export const DEV_CAP_KEY_MINT = 'key.mint'
export const DEV_CAP_SCOPE_APPLY = 'scope.apply'

// A row written before the approval flow existed carries an empty status and is
// a live application, so the portal must read it as approved rather than as an
// unknown state.
export const DEV_APP_REVIEW_LABELS: Record<string, string> = {
  approved: '已通过',
  pending: '待审核',
  declined: '未通过'
}

export const DEV_APP_REVIEW_COLORS: Record<
  string,
  'success' | 'warning' | 'danger'
> = {
  approved: 'success',
  pending: 'warning',
  declined: 'danger'
}

export const isAppUnderReview = (status: string): boolean =>
  status === 'pending' || status === 'declined'

export const DEV_DISABLED_HINT = '该功能当前已由平台关闭'

export const MAX_APPS_PER_ACCOUNT = 5
export const MAX_ACTIVE_KEYS_PER_APP = 5

export const API_BASE_URL = 'https://api.nextmoe.dev/v2'

export const MCP_ENDPOINT = 'https://mcp.nextmoe.dev/mcp'
