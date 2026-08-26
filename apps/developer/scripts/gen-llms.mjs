// Writes the LLM-facing half of the portal docs into public/: llms.txt (a
// compact index that fits any context window), llms-full.txt (every operation
// inlined), and a clean Markdown twin for every /docs route, served at
// `<route>.md`. The "复制页面" control and AI tools fetch those twins.
//
// Called by sync-specs.mjs from the SAME DocsModel the reference pages render,
// so the Markdown cannot drift from the page: there is no second description of
// an operation to keep in sync.
//
// Output must be byte-identical for identical input — no timestamps, no
// randomness, no Date.now(). The CI drift gate diffs the committed files.

import { mkdirSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'

import {
  API_HOST,
  ATTRIBUTION_NOTE,
  MCP_ENDPOINT,
  SITE_URL,
  SOURCES
} from '../shared/brand.mjs'
import { MCP_TOOLS } from '../shared/mcp-tools.mjs'

const AUTH_MODEL = [
  '应用密钥（`Authorization: Bearer nmk_live_…`）——在 ' +
    `${SITE_URL} 控制台自助创建应用与密钥，无需申请；自助可勾选的 scope 只有 catalog:read。` +
    '/v2 只收 nmk_ 前缀的密钥，v1 两代都收。',
  '用户访问令牌（`Authorization: Bearer <access token>`）——/v2/me 与 /v2/moderation（以及 v1 的游玩时长、编辑提案）' +
    '读写的是某个用户自己的东西，用该用户经 OAuth 授权码 + PKCE 授权后的令牌，不是应用密钥。',
  '资讯面不再是授权制（2026-08-25 退役）：/v1/news 只要一把有效密钥，' +
    '任意 scope 均可；news:read 不再存在申请或审批。/v2/news 则匿名即可调。',
  'store:read 自 2026-08-26 起自助：铸密钥时勾选即可，无需申请；没有它调 /v1/store 一律 403。',
  '`/v2/news`、`/v2/vocabularies`、`/v2/problems`、`/v2/catalog/stats` 与 ' +
    '`/v2/catalog/schemas/{object}`（以及 `/v1/catalog/stats`）不要任何凭据，匿名即可调。'
]

const mdPath = (route) => (route === '/' ? '/index.md' : `${route}.md`)

const scopeLine = (op) => {
  if (op.auth?.kind === 'none') return '无需凭据'
  return op.scope || '无需凭据'
}

const paramTable = (params) => {
  if (!params.length) return ['无参数。', '']
  const rows = params.map((p) => {
    const type = p.format ? `${p.type} (${p.format})` : p.type
    const doc = (p.doc ?? '').replace(/\|/g, '\\|').replace(/\n+/g, ' ')
    const enums = p.enum?.length ? ` 取值：${p.enum.join(' \\| ')}` : ''
    return `| \`${p.name}\` | ${p.in} | ${p.required ? '是' : '否'} | ${type} | ${doc}${enums} |`
  })
  return [
    '| 参数 | 位置 | 必填 | 类型 | 说明 |',
    '| --- | --- | --- | --- | --- |',
    ...rows,
    ''
  ]
}

const operationSection = (face, op, heading) => {
  const lines = [
    `${heading} ${op.method.toUpperCase()} ${op.path}`,
    '',
    op.summary,
    ''
  ]
  if (op.description) lines.push(op.description, '')
  lines.push(`- 所属 API：${face.name}（${face.prefix}）`)
  lines.push(`- 鉴权：${(op.auth ?? face.auth).display}`)
  lines.push(`- scope：${scopeLine(op)}`)
  lines.push('')
  lines.push(...paramTable(op.params))
  lines.push('```bash', op.curl, '```', '')
  return lines
}

const faceOperations = (face) =>
  face.groups.flatMap((g) => g.operations.map((op) => ({ group: g, op })))

const header = (title) => [
  `# ${title}`,
  '',
  '> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，' +
    'NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。',
  '',
  `- Base URL：${API_HOST}`,
  `- 文档：${SITE_URL}/docs`,
  `- MCP 端点：${MCP_ENDPOINT}`,
  `- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。`,
  '',
  `**署名**：${ATTRIBUTION_NOTE}`,
  ''
]

const sourceSection = () => [
  '## 数据来源（六源）',
  '',
  ...SOURCES.map(([name, body]) => `- ${name}：${body}`),
  '',
  '除六个上游站点外，未萌生态站点自己产出的条目、译名与整理，以及用户经编辑提案提交的修改，同样进入这一份记录。',
  ''
]

const authSection = () => [
  '## 鉴权模型',
  '',
  ...AUTH_MODEL.map((line) => `- ${line}`),
  ''
]

const buildLlmsTxt = (model) => {
  const lines = [...header('NextMoe 开放 API')]
  lines.push(...sourceSection())
  lines.push(...authSection())
  lines.push(`## ${model.faces.length} 个 API`, '')
  for (const face of model.faces) {
    const count = faceOperations(face).length
    lines.push(
      `- [${face.name}](${SITE_URL}/docs/${face.key}.md)：\`${face.prefix}\`，` +
        `${count} 个端点，凭据 ${face.auth.display}。`
    )
  }
  lines.push('')
  lines.push('## 页面 Markdown 索引', '')
  lines.push(
    '每个文档页都有一份干净的 Markdown 孪生，路径规则是在路由后加 `.md`：',
    ''
  )
  lines.push(`- ${SITE_URL}/index.md — 平台总览`)
  lines.push(`- ${SITE_URL}/docs.md — 文档首页（别名 ${SITE_URL}/docs/index.md）`)
  for (const face of model.faces) {
    lines.push(`- ${SITE_URL}/docs/${face.key}.md — ${face.name}`)
  }
  lines.push(`- ${SITE_URL}/docs/mcp.md — AI / MCP 接入`)
  lines.push(
    `- ${SITE_URL}/docs/<face>/<operationId>.md — 单个端点`,
    ''
  )
  lines.push(`全量参考（每个端点的参数与 curl）见 ${SITE_URL}/llms-full.txt。`, '')
  return lines.join('\n')
}

const buildLlmsFull = (model) => {
  const lines = [...header('NextMoe 开放 API —— 面向 LLM 的全量文档')]
  lines.push(
    `本文件内联全部 ${model.faces.reduce((n, f) => n + faceOperations(f).length, 0)} 个端点。` +
      `紧凑索引见 ${SITE_URL}/llms.txt。`,
    ''
  )
  lines.push(...sourceSection())
  lines.push(...authSection())
  for (const face of model.faces) {
    lines.push('---', '', `# ${face.name}`, '')
    lines.push(`- 路径前缀：\`${face.prefix}\``)
    lines.push(`- 凭据：${face.auth.display}（${face.auth.note}）`)
    lines.push(`- 端点数：${faceOperations(face).length}`)
    lines.push('')
    if (face.notes?.length) {
      for (const note of face.notes) lines.push(`> ${note}`, '')
    }
    for (const group of face.groups) {
      if (face.groups.length > 1) lines.push(`## ${group.label}`, '')
      for (const op of group.operations) {
        lines.push(...operationSection(face, op, '###'))
      }
    }
  }
  lines.push('---', '', ...mcpSection())
  return lines.join('\n')
}

const mcpSection = () => {
  const lines = [
    '# AI / MCP 接入',
    '',
    `NextMoe 开放 API 同时以 MCP（Model Context Protocol）server 暴露：端点 ${MCP_ENDPOINT}，` +
      'Streamable HTTP、stateless，带上同一把 API 密钥即可。它是一层纯透传适配——' +
      '每次工具调用就是一次对公开 /v2 GET 的请求，鉴权、限流、配额与用量与直连毫无区别。' +
      '工具名就是 OpenAPI operationId，清单由 cmd/gen-v2-portal 从同一份 v2 spec 生成。',
    '',
    `## 工具（${MCP_TOOLS.length} 个）`,
    ''
  ]
  for (const tool of MCP_TOOLS) {
    lines.push(
      `- \`${tool.name}\` \`${tool.method} ${tool.path}\`${tool.needsKey ? '' : '（无需密钥）'}：${tool.desc}`
    )
  }
  lines.push('')
  return lines
}

const pageFooter = (route) => [
  '---',
  `本页来源 · NextMoe 开发者平台 · ${SITE_URL}${route}`,
  ''
]

const buildPages = (model) => {
  const pages = new Map()

  pages.set('/', [
    ...header('NextMoe 开发者平台'),
    ...sourceSection(),
    ...authSection(),
    '## 三步开始',
    '',
    '1. 用生态账号（NextMoe / 鲲 Galgame）登录门户，不必另外注册开发者身份。',
    '2. 在控制台创建一个应用（每个账号最多 5 个），拿到独立的配额与用量视图。',
    '3. 生成密钥并妥善保存（只显示一次），带上它请求只读端点；要读写用户自己的游玩记录或提交编辑提案，改用那个用户授权后的访问令牌。',
    '',
    `完整端点参考见 ${SITE_URL}/llms-full.txt。`,
    '',
    ...pageFooter('/')
  ])

  const docsIndex = [
    ...header('API 文档'),
    `## ${model.faces.length} 个 API`,
    ''
  ]
  for (const face of model.faces) {
    docsIndex.push(
      `- [${face.name}](${SITE_URL}/docs/${face.key}.md) — \`${face.prefix}\`，` +
        `${faceOperations(face).length} 个端点，${face.auth.display}`
    )
  }
  docsIndex.push('', ...authSection(), ...pageFooter('/docs'))
  pages.set('/docs', docsIndex)

  for (const face of model.faces) {
    const lines = [
      ...header(face.name),
      `- 路径前缀：\`${face.prefix}\``,
      `- 凭据：${face.auth.display}（${face.auth.note}）`,
      `- 端点数：${faceOperations(face).length}`,
      ''
    ]
    if (face.notes?.length) {
      lines.push('## 使用须知', '')
      for (const note of face.notes) lines.push(`- ${note}`)
      lines.push('')
    }
    lines.push('## 端点', '')
    for (const group of face.groups) {
      if (face.groups.length > 1) lines.push(`### ${group.label}`, '')
      for (const op of group.operations) {
        lines.push(
          `- \`${op.method.toUpperCase()} ${op.path}\` — ${op.summary}` +
            ` [详情](${SITE_URL}/docs/${face.key}/${op.id}.md)`
        )
      }
      lines.push('')
    }
    lines.push(...pageFooter(`/docs/${face.key}`))
    pages.set(`/docs/${face.key}`, lines)

    for (const { op } of faceOperations(face)) {
      const route = `/docs/${face.key}/${op.id}`
      pages.set(route, [
        ...header(`${op.summary || op.id} · ${face.name}`),
        ...operationSection(face, op, '##'),
        ...pageFooter(route)
      ])
    }
  }

  pages.set('/docs/mcp', [
    ...header('AI / MCP 接入'),
    ...mcpSection().slice(2),
    ...pageFooter('/docs/mcp')
  ])

  return pages
}

export const writeLlmArtifacts = (model, publicDir) => {
  const write = (relative, content) => {
    const out = join(publicDir, relative)
    mkdirSync(dirname(out), { recursive: true })
    writeFileSync(out, content.endsWith('\n') ? content : `${content}\n`)
  }

  write('llms.txt', buildLlmsTxt(model))
  write('llms-full.txt', buildLlmsFull(model))

  const pages = buildPages(model)
  for (const [route, lines] of pages) {
    write(mdPath(route).slice(1), lines.join('\n'))
  }

  // `<route>.md` is the one rule — it is what CopyPage.vue derives from the
  // current path, so /docs is served at docs.md. `docs/index.md` is the shape a
  // reader guesses for a section front page, and a guessed 404 costs an agent a
  // whole turn; the alias comes from the same builder, so it cannot drift.
  write('docs/index.md', pages.get('/docs').join('\n'))

  return pages.size + 3
}
