
export const CATALOG_FILTER_ALL = -1

export const CATALOG_ENTITY_TYPES: Record<number, string> = {
  0: '人物',
  1: '名义',
  2: '组织',
  3: '厂牌',
  4: '角色',
  5: '作品',
  6: '版本'
}

export const CATALOG_ENTITY_TYPE = {
  creditName: 1
} as const

export const CANDIDATE_STATUS = {
  pending: 0,
  accepted: 1,
  rejected: 2,
  deferred: 3,
  needsManual: 4
} as const

export const WORK_STATUS = {
  LIVE: 0,
  STUB: 1,
  MERGED: 2,
  QUARANTINE: 3
} as const

export const WORK_STATUS_LABELS: Record<number, string> = {
  0: '公开',
  1: '未达标',
  2: '已合并',
  3: '隔离中'
}

export const CANDIDATE_STATUS_LABELS: Record<number, string> = {
  0: '待处理',
  1: '已接受',
  2: '已拒绝',
  3: '已搁置',
  4: '待人工'
}

export const CANDIDATE_REASON_LABELS: Record<number, string> = {
  0: '共享外部 ID',
  1: '规范化同名',
  2: '名称近似',
  3: '导入器建议',
  4: 'LLM 建议',
  5: '别名声明'
}

export const PROPOSAL_STATUS = {
  open: 0,
  approved: 1,
  executed: 2,
  rejected: 3,
  withdrawn: 4
} as const

export const PROPOSAL_STATUS_LABELS: Record<number, string> = {
  0: '待审批',
  1: '冷静期',
  2: '已执行',
  3: '已拒绝',
  4: '已撤回'
}

export type CatalogChipColor =
  | 'default'
  | 'primary'
  | 'secondary'
  | 'success'
  | 'warning'
  | 'danger'
  | 'info'

export const CANDIDATE_STATUS_COLORS: Record<number, CatalogChipColor> = {
  0: 'warning',
  1: 'success',
  2: 'danger',
  3: 'default',
  4: 'secondary'
}

export const PROPOSAL_STATUS_COLORS: Record<number, CatalogChipColor> = {
  0: 'warning',
  1: 'info',
  2: 'success',
  3: 'danger',
  4: 'default'
}

export const MATCHED_BY_KINDS: ReadonlyArray<{
  prefix: string
  label: string
  color: CatalogChipColor
}> = [
  { prefix: 'rule:', label: '规则', color: 'info' },
  { prefix: 'import:', label: '导入', color: 'default' },
  { prefix: 'human:', label: '人工', color: 'success' }
]

export const CATALOG_SOURCE_LABELS: Record<number, string> = {
  1: '人工',
  2: 'VNDB',
  3: 'Bangumi',
  4: 'DLsite',
  5: 'ErogameScape',
  6: 'AniList',
  7: 'MAL',
  8: 'Steam',
  9: '官网',
  10: 'Twitter',
  11: 'Pixiv'
}

export const CATALOG_IMAGE_REF_KIND_LABELS: Record<string, string> = {
  work_cover: '封面',
  work_screenshot: '截图',
  character_bust: '角色胸像',
  character_figure: '角色立绘',
  label_logo: '会社 logo',
  person_photo: '人物照片'
}
