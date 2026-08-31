export interface GuideTocEntry {
  id: string
  text: string
  depth: 2 | 3
}

export interface GuideMeta {
  slug: string
  title: string
  /** Section label shown above the page title. */
  eyebrow: string
  description: string
}

/** One compiled page from apps/developer/docs/*.md. */
export interface GuidePage extends GuideMeta {
  route: string
  toc: GuideTocEntry[]
  /** Pre-highlighted at build time — no markdown or Shiki in the client bundle. */
  html: string
}

export interface GuideNavLink {
  to: string
  label: string
}

export interface GuideNavSection {
  key: string
  label: string
  links: GuideNavLink[]
}
