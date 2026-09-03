// Builds the client-side search index the ⌘K palette queries — one flat list of
// every routable page on the portal, with the text a reader would search for.
//
// Written from the SAME model sync-specs.mjs hands to gen-llms.mjs, so a page
// that exists has an entry by construction. The HTML pages render a Chinese
// overlay over an English contract, and a reader may know either half, so each
// entry carries both: `t`/`d` are what the palette shows, `b` is the haystack
// and holds the English original too.
//
// Emitted as JSON rather than TS because it is data the palette lazy-imports as
// its own chunk on first open — nothing else in the app may pull it in.
import { existsSync } from 'node:fs'
import { dirname, join as joinPath } from 'node:path'
import { fileURLToPath } from 'node:url'

import { API_HOST, SITE_URL } from '../shared/brand.mjs'

const __dirname = dirname(fileURLToPath(import.meta.url))
const PAGES_DIR = joinPath(__dirname, '..', 'app/pages')

// Markdown syntax is noise in a haystack and worse in a snippet. Fences, inline
// code and link targets go; the text inside them stays. `_` is deliberately NOT
// stripped with the other emphasis marks — in this corpus it is almost never
// emphasis and almost always an identifier, and stripping it turned nmk_live_
// and include_total, the two things people actually search for, into word soup.
const plain = (markdown) =>
  markdown
    .replace(/```[\s\S]*?```/g, (block) =>
      block.replace(/```[a-z]*\n?/g, '').replace(/\n/g, ' ')
    )
    .replace(/^\s*#{1,6}\s+/gm, '')
    .replace(/\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]/g, '')
    .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/[`*>|]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()

const join = (...parts) => parts.filter(Boolean).join(' ')

// Site pages the docs model knows nothing about. Hand-maintained on purpose —
// they are the app's own routes, not anything a generator can see — so each one
// is checked against app/pages below: a palette hit that 404s is worse than a
// page that cannot be found at all.
const SITE_PAGES = [
  {
    r: '/',
    t: 'NextMoe 开发者平台',
    s: '站点',
    d: 'ACGN 数据，以此为准',
    b: 'NextMoe 未萌 开放 API 首页 landing home'
  },
  {
    r: '/explore',
    t: '数据浏览',
    s: '站点',
    d: '不写代码先看数据：搜索作品、角色、人物、厂牌与标签',
    b: 'explore 浏览 搜索 试用 playground'
  },
  {
    r: '/dashboard',
    t: '控制台',
    s: '站点',
    d: '创建应用、铸造密钥、查看配额',
    b: 'dashboard 控制台 应用 密钥 key app'
  },
  {
    r: '/dashboard/usage',
    t: '用量',
    s: '站点',
    d: '按应用与端点看调用量、错误率与限流',
    b: 'usage 用量 统计 配额 quota 限流'
  },
  {
    r: '/dashboard/store',
    t: '分销链接',
    s: '站点',
    d: '商店联盟链接的铸造与点击统计',
    b: 'store 商店 购买 联盟 affiliate dlsite'
  },
  {
    r: '/docs',
    t: 'API 文档',
    s: '文档',
    d: '从这里开始：指南、端点参考、错误码与词表',
    b: 'docs 文档 概览 overview'
  },
  {
    r: '/docs/mcp',
    t: 'AI / MCP 接入',
    s: '文档',
    d: '把 NextMoe 作为 MCP server 接进 Claude、Cursor 等客户端',
    b: `MCP model context protocol AI 助手 ${SITE_URL}/docs/mcp`
  },
  {
    r: '/problems',
    t: '错误码',
    s: '文档',
    d: 'RFC 9457 problem types 注册表',
    b: 'problem types 错误 错误码 registry RFC 9457'
  },
  {
    r: '/docs/vocabularies',
    t: '词表',
    s: '文档',
    d: '全部开放与封闭词表及其成员',
    b: `vocabularies 词表 枚举 enum ${API_HOST}/v2/vocabularies`
  }
]

export const buildSearchIndex = ({
  model,
  guides,
  problems,
  problemsZh,
  vocabularies,
  vocabulariesZh,
  t
}) => {
  const entries = [...SITE_PAGES]

  for (const guide of guides) {
    entries.push({
      r: guide.route,
      t: guide.title,
      s: `指南 · ${guide.eyebrow}`,
      d: guide.description,
      b: join(guide.description, plain(guide.markdown)),
      h: guide.toc.map((h) => ({ i: h.id, t: h.text }))
    })
  }

  for (const face of model.faces) {
    entries.push({
      r: `/docs/${face.key}`,
      t: face.name,
      s: '端点参考',
      d: `${face.prefix} · ${face.groups.reduce((n, g) => n + g.operations.length, 0)} 个端点`,
      b: join(face.key, face.prefix, face.label, face.name)
    })
    for (const group of face.groups) {
      for (const op of group.operations) {
        entries.push({
          r: `/docs/${face.key}/${op.id}`,
          t: t(op.summary) || op.id,
          s: `端点 · ${group.label}`,
          d: `${op.method.toUpperCase()} ${op.path}`,
          b: join(
            op.id,
            op.path,
            op.method,
            op.summary,
            op.description,
            t(op.description),
            op.scope,
            op.params.map((p) => p.name).join(' ')
          )
        })
      }
    }
  }

  for (const p of problems) {
    const zh = problemsZh.entries[p.code]
    entries.push({
      r: `/problems/${p.domain}/${p.kebab}`,
      t: zh?.title ?? p.title,
      s: `错误码 · ${problemsZh.domains[p.domain] ?? p.domain}`,
      d: `${p.code} · HTTP ${p.status}`,
      b: join(p.code, p.title, p.description, zh?.description, p.domain)
    })
  }

  for (const vocab of vocabularies) {
    const meta = vocabulariesZh.meta[vocab.name]
    const zh = vocabulariesZh.values[vocab.name] ?? {}
    entries.push({
      r: `/docs/vocabularies/${vocab.name}`,
      t: meta?.label ?? vocab.name,
      s: `词表 · ${vocab.closed ? '封闭' : '开放'}`,
      d: `${vocab.name} · ${vocab.values.length} 个成员`,
      b: join(
        vocab.name,
        meta?.summary,
        vocab.values
          .map((v) =>
            join(
              v.value,
              v.display_name,
              v.description,
              zh[v.value]?.displayName,
              zh[v.value]?.description
            )
          )
          .join(' ')
      )
    })
  }

  const seen = new Set()
  for (const e of entries) {
    if (seen.has(e.r)) {
      throw new Error(`search index: duplicate route ${e.r}`)
    }
    seen.add(e.r)
  }

  const missing = SITE_PAGES.map((p) => p.r).filter((route) => {
    const rel = route === '/' ? 'index' : route.slice(1)
    return (
      !existsSync(joinPath(PAGES_DIR, `${rel}.vue`)) &&
      !existsSync(joinPath(PAGES_DIR, rel, 'index.vue'))
    )
  })
  if (missing.length) {
    throw new Error(
      `search index: SITE_PAGES routes with no page under app/pages: ${missing.join(', ')}`
    )
  }

  return entries
}
