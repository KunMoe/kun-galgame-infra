export type SettingKind =
  | 'bool'
  | 'int'
  | 'float'
  | 'string'
  | 'enum'
  | 'string_list'

export type SettingSource = 'db' | 'default'

export type SettingValue = boolean | number | string | string[]

export interface SettingsOverrideView {
  value: SettingValue
  version: number
  updated_by_user_id: number
  updated_by_name: string
  note: string
  updated_at: string
}

export interface SettingsKeyView {
  key: string
  kind: SettingKind
  desc_en: string
  desc_zh: string
  default: SettingValue
  env_var?: string
  enum?: string[]
  min?: number
  max?: number
  pattern?: string
  effective: SettingValue
  source: SettingSource
  override: SettingsOverrideView | null
}

export interface SettingsDomainView {
  name: string
  title_zh: string
  keys: SettingsKeyView[]
}

export interface SettingsOverview {
  domains: SettingsDomainView[]
  writable: boolean
}

export type SettingsAuditAction = 'set' | 'reset'

export interface SettingsAuditEntry {
  id: number
  actor_user_id: number
  actor_name: string
  action: SettingsAuditAction
  key: string
  old_value: SettingValue | null
  new_value: SettingValue | null
  note: string
  created_at: string
}
