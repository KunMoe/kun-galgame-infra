import type { StoreLinkKind } from '~~/shared/types/store'

export const STORE_LINK_KIND_LABELS: Record<StoreLinkKind, string> = {
  purchase: '购买',
  coupon: '优惠券'
}

export const STORE_LINK_KIND_COLORS: Record<
  StoreLinkKind,
  'primary' | 'secondary'
> = {
  purchase: 'primary',
  coupon: 'secondary'
}

export const STORE_USAGE_DAY_OPTIONS = [7, 30, 90]

export const STORE_SCOPE = 'store:read'

export const STORE_LINK_PAGE_SIZE = 20

export const STORE_LINK_KIND_FILTERS = [
  { value: 'all', label: '全部类型' },
  { value: 'purchase', label: '购买' },
  { value: 'coupon', label: '优惠券' }
] as const

export const STORE_LINK_SORTS = [
  { value: 'uniques', label: '按去重点击' },
  { value: 'total', label: '按总点击' },
  { value: 'target', label: '按商品 / 活动' }
] as const

export type StoreLinkSort = (typeof STORE_LINK_SORTS)[number]['value']

/** How many links the bar chart above the table ranks. */
export const STORE_TOP_LINKS = 8
