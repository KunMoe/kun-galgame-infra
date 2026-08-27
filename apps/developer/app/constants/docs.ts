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
