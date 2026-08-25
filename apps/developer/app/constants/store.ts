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
