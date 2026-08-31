export const SITE_NAV = [
  { to: '/', label: '首页', icon: 'lucide:home' },
  { to: '/docs', label: 'API 文档', icon: 'lucide:book-open' },
  { to: '/explore', label: '数据浏览', icon: 'lucide:compass' },
  { to: '/dashboard', label: '控制台', icon: 'lucide:layout-dashboard' }
] as const

// Shared by the dashboard layout's desktop rail and the mobile drawer — below
// lg the rail is gone, and these three routes had no other way in.
export const DASHBOARD_NAV = [
  {
    to: '/dashboard',
    label: '我的应用',
    icon: 'lucide:boxes',
    match: ['/dashboard', '/apps']
  },
  {
    to: '/usage',
    label: '用量',
    icon: 'lucide:chart-column',
    match: ['/usage']
  },
  {
    to: '/store',
    label: '分销链接',
    icon: 'lucide:shopping-bag',
    match: ['/store']
  },
  {
    to: '/docs',
    label: 'API 文档',
    icon: 'lucide:book-open',
    match: ['/docs']
  }
] as const

export const isDashboardRoute = (path: string) =>
  DASHBOARD_NAV.some((item) =>
    item.match.some(
      (m) => m !== '/docs' && (path === m || path.startsWith(`${m}/`))
    )
  )
