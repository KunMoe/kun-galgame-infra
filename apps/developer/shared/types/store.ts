export type StoreLinkKind = 'purchase' | 'coupon'

export interface StoreUsageDay {
  day: string
  total: number
  uniques: number
}

export interface StoreUsageApp {
  client_id: string
  name: string
  links: number
  total: number
  uniques: number
}

export interface StoreUsageLink {
  client_id: string
  app_name: string
  kind: StoreLinkKind
  product_id: string | null
  campaign_id: number | null
  total: number
  uniques: number
}

export interface StoreUsageSummary {
  days: number
  since: string
  until: string
  total: number
  uniques: number
  link_count: number
  daily: StoreUsageDay[]
  by_app: StoreUsageApp[]
  by_link: StoreUsageLink[]
}
