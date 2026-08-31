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
  v2: {
    icon: 'lucide:sparkles',
    label: 'API v2',
    tagline:
      'NextMoe 公开 API v2，正式公开面。无信封、RFC 9457 错误、字符串 id、keyset 游标。应用密钥是 nmk_live_（CRC32 校验位），任何应用都能在门户自助铸造，无需申请。'
  }
}

export const docsFaceLabel = (face: string): string =>
  DOCS_FACE_META[face as DocsFaceKey]?.label ?? face

// The reference half of the docs sidebar. The guide half is generated —
// app/generated/guides-nav.ts, ordered by scripts/guides.mjs.
export const DOCS_REFERENCE_NAV = [
  { to: '/docs/vocabularies', label: '词表' },
  { to: '/problems', label: '错误码' },
  { to: '/docs/mcp', label: 'AI / MCP 接入' }
] as const

export const DOCS_GUIDE_ICONS: Record<string, string> = {
  '/docs/quickstart': 'lucide:rocket',
  '/docs/authentication': 'lucide:key-round',
  '/docs/concepts': 'lucide:network',
  '/docs/design': 'lucide:drafting-compass',
  '/docs/conventions': 'lucide:file-json',
  '/docs/pagination': 'lucide:layers',
  '/docs/shaping': 'lucide:scissors',
  '/docs/errors': 'lucide:octagon-alert',
  '/docs/rate-limits': 'lucide:gauge',
  '/docs/caching': 'lucide:database-zap',
  '/docs/versioning': 'lucide:git-branch',
  '/docs/example': 'lucide:route',
  '/docs/mirror': 'lucide:refresh-cw',
  '/docs/user-data': 'lucide:user-round-cog',
  '/docs/best-practices': 'lucide:list-checks'
}

// The three integrations people actually build. Each names the guide that
// carries the whole recipe, so the overview stays a map rather than a summary.
export const DOCS_SCENARIOS = [
  {
    to: '/docs/example',
    icon: 'lucide:search',
    title: '按需查询',
    body: '做一个展示站或机器人：搜索、详情、反查外部 id，全部按需调用，配合 ETag 缓存即可。'
  },
  {
    to: '/docs/mirror',
    icon: 'lucide:refresh-cw',
    title: '镜像到自己的库',
    body: '要在自己的 SQL 里过滤排序：冷启动一次全量，之后订增量信道保持新鲜。'
  },
  {
    to: '/docs/user-data',
    icon: 'lucide:user-round-check',
    title: '代表用户读写',
    body: '游玩时长、编辑提案、认领与投票：走 OAuth 拿用户令牌，而不是应用密钥。'
  }
] as const
