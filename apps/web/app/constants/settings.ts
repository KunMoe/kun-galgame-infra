import type {
  SettingKind,
  SettingSource,
  SettingValue,
  SettingsAuditAction
} from '~~/shared/types/settings'

export type SettingsChipColor =
  | 'default'
  | 'primary'
  | 'secondary'
  | 'success'
  | 'warning'
  | 'danger'
  | 'info'

export const KIND_LABELS: Record<SettingKind, string> = {
  bool: '布尔',
  int: '整数',
  float: '小数',
  string: '文本',
  enum: '枚举',
  string_list: '列表'
}

export const SOURCE_LABELS: Record<SettingSource, string> = {
  db: '已覆盖',
  default: '默认'
}

export const SOURCE_COLORS: Record<SettingSource, SettingsChipColor> = {
  db: 'primary',
  default: 'default'
}

export const AUDIT_ACTION_LABELS: Record<SettingsAuditAction, string> = {
  set: '设置',
  reset: '撤销覆盖'
}

export const AUDIT_ACTION_COLORS: Record<
  SettingsAuditAction,
  SettingsChipColor
> = {
  set: 'success',
  reset: 'warning'
}

export const AUDIT_LIST_LIMIT = 20

export const PROPAGATION_NOTE = '保存后所有服务在 30 秒内生效'

export const ENV_FLOOR_NOTE =
  '若所属服务的环境里设置了该变量,它会覆盖默认值,直到这里设置了覆盖值'

export const formatSettingValue = (
  kind: SettingKind,
  v: SettingValue | null | undefined
): string => {
  if (v === null || v === undefined) {
    return '—'
  }
  if (kind === 'bool') {
    return v === true ? '开' : '关'
  }
  if (kind === 'string_list') {
    if (!Array.isArray(v) || v.length === 0) {
      return '(空)'
    }
    return v.join(', ')
  }
  if (typeof v === 'string') {
    return v === '' ? '(空)' : v
  }
  return String(v)
}
