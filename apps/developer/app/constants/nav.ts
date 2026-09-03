export const SITE_NAV = [
  { to: '/', label: '首页', icon: 'lucide:home' },
  { to: '/docs', label: 'API 文档', icon: 'lucide:book-open' },
  { to: '/explore', label: '数据浏览', icon: 'lucide:compass' },
  { to: '/dashboard', label: '控制台', icon: 'lucide:layout-dashboard' }
] as const

export interface DashboardNavItem {
  to: string
  label: string
  icon: string
  // `/dashboard` is a prefix of every other console route, so the apps entry
  // cannot match by prefix — it matches its own path exactly and picks the
  // detail pages up through `also`.
  exact: boolean
  also: readonly string[]
}

// Shared by the dashboard layout's desktop rail and the mobile drawer — below
// lg the rail is gone, and these routes had no other way in.
export const DASHBOARD_NAV: readonly DashboardNavItem[] = [
  {
    to: '/dashboard',
    label: '我的应用',
    icon: 'lucide:boxes',
    exact: true,
    also: ['/dashboard/apps']
  },
  {
    to: '/dashboard/usage',
    label: '用量',
    icon: 'lucide:chart-column',
    exact: false,
    also: []
  },
  {
    to: '/dashboard/store',
    label: '分销链接',
    icon: 'lucide:shopping-bag',
    exact: false,
    also: []
  },
  {
    to: '/docs',
    label: 'API 文档',
    icon: 'lucide:book-open',
    exact: false,
    also: []
  }
]

const covers = (base: string, path: string) =>
  path === base || path.startsWith(`${base}/`)

export const isDashboardNavActive = (item: DashboardNavItem, path: string) =>
  (item.exact ? path === item.to : covers(item.to, path)) ||
  item.also.some((base) => covers(base, path))

export const isDashboardRoute = (path: string) => covers('/dashboard', path)

export const ACCOUNT_NAV = [
  { to: '/docs/v2', label: '端点参考', icon: 'lucide:list' },
  { to: '/docs/mcp', label: '配置 MCP', icon: 'lucide:plug' },
  { to: '/dashboard/usage', label: '我的用量', icon: 'lucide:chart-column' },
  { to: '/dashboard', label: '我的应用', icon: 'lucide:boxes' },
  { to: '/dashboard/store', label: '分销链接', icon: 'lucide:shopping-bag' }
] as const
