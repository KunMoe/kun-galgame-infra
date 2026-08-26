import type { DocsMethod, DocsFaceKey } from '~~/shared/types/docs'

export const DOCS_METHOD_BADGE: Record<DocsMethod, string> = {
  get: 'bg-success-50 text-success-600',
  post: 'bg-primary-50 text-primary-600',
  put: 'bg-warning-50 text-warning-600',
  patch: 'bg-warning-50 text-warning-600',
  delete: 'bg-danger-50 text-danger-600'
}

export const DOCS_FACE_META: Record<
  DocsFaceKey,
  { icon: string; label: string; tagline: string; badge?: string }
> = {
  catalog: {
    icon: 'lucide:network',
    label: '目录数据',
    tagline:
      '作品、角色、厂牌、制作人员的统一条目库。同一部作品在六个源各有一个页面，我们把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上这个答案取自哪个源。'
  },
  playtime: {
    icon: 'lucide:timer',
    label: '游玩时长',
    tagline:
      '用户自己的游玩时长：上报（单条 / 用外部 id 定位 / 批量）与回拉。用用户访问令牌，不是 API 密钥。任何已开通用户登录的应用都可以调用，不需要 playtime:read / playtime:write；一个用户只读写得到自己的记录。'
  },
  edit: {
    icon: 'lucide:pencil',
    label: '编辑提案',
    tagline:
      '往目录里提交修改：读字段表与当前值，创建、列出、查看、撤回自己的提案。用用户令牌，需 catalog:edit；第三方应用只能提案，不能自己批准。'
  },
  news: {
    icon: 'lucide:newspaper',
    label: '资讯',
    tagline:
      '合作媒体的 Galgame 资讯索引：标题、摘要、题图与回源链接，正文不下发。这个 v1 面的密钥须带 news:read，该权限授权制——登录门户后在控制台申请，批准后即可自助勾选；同一份索引在 /v2/news 上无需任何凭据。',
    badge: '授权制'
  },
  v2: {
    icon: 'lucide:sparkles',
    label: 'API v2',
    tagline:
      'NextMoe 公开 API v2，正式公开面。无信封、RFC 9457 错误、字符串 id、keyset 游标。应用密钥是 nmk_live_（CRC32 校验位），任何应用都能在门户自助铸造，无需申请。'
  }
}

export const docsFaceLabel = (face: string): string =>
  DOCS_FACE_META[face as DocsFaceKey]?.label ?? face
