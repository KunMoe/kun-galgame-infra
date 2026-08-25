// Face definitions and portal information architecture for the /docs reference.
//
// Split out of sync-specs.mjs when that file also grew the llms.txt / page-Markdown
// generators: this half is pure declaration (which prefix is which face, which
// credential it takes, which group each operation lands in), the other half is
// the machinery that reads the specs. Both generators import from here, so the
// reference pages and the LLM-facing text can never describe different faces.
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))
export const REPO_ROOT = join(__dirname, '..', '..', '..')

export const API_HOST = 'https://api.nextmoe.dev'

export const PUBLIC_SPEC = join(REPO_ROOT, 'docs/catalog/public-openapi.yaml')
export const CATALOG_SPEC = join(REPO_ROOT, 'docs/catalog/openapi.yaml')
export const NEWS_SPEC = join(REPO_ROOT, 'docs/news/public-openapi.yaml')
export const V2_SPEC = join(REPO_ROOT, 'docs/catalog/v2-openapi.yaml')

// A face is a path prefix plus the credential that prefix accepts. The spec
// itself carries neither: OpenAPI security schemes are not emitted by the
// public gen target, and scope lives in prose. Both are derived here, from the
// prefix and the method, and they are the only place the reference pages learn
// which credential to print.
export const FACES = [
  {
    key: 'catalog',
    label: '目录数据',
    name: '目录数据 API（只读）',
    file: PUBLIC_SPEC,
    prefix: '/v1/catalog',
    scope: () => 'catalog:read',
    auth: {
      kind: 'api_key',
      curl: 'Authorization: Bearer nm_live_<YOUR_KEY>',
      display: 'Authorization: Bearer nm_live_…',
      note: '机器 API 密钥,服务端持有'
    }
  },
  {
    key: 'playtime',
    label: '游玩时长',
    name: '游玩时长 API',
    file: PUBLIC_SPEC,
    prefix: '/v1/playtime',
    scope: () => '',
    auth: {
      kind: 'user_token',
      curl: 'Authorization: Bearer <ACCESS_TOKEN>',
      display: 'Authorization: Bearer <用户访问令牌>',
      note: '用户访问令牌。任何已开通用户登录的应用都可以调用，不需要 playtime:read / playtime:write'
    }
  },
  {
    key: 'edit',
    label: '编辑提案',
    name: '编辑提案 API',
    file: CATALOG_SPEC,
    prefix: '/api/v1/user/catalog/edit',
    include: [
      'getEditSchemaUser',
      'getEditSnapshotUser',
      'createEditProposalUser',
      'listEditProposalsUser',
      'getEditProposalUser',
      'withdrawEditProposalUser'
    ],
    scope: () => 'catalog:edit',
    auth: {
      kind: 'user_token',
      curl: 'Authorization: Bearer <ACCESS_TOKEN>',
      display: 'Authorization: Bearer <用户访问令牌>',
      note: '用户授权后的访问令牌(OAuth 授权码 + PKCE),需 catalog:edit scope'
    },
    notes: [
      '第三方应用的令牌恒为「只提案」姿态：审核、自动合入、撤销他人提案永不可；schema 投影对它 can_review=false。',
      '每用户未决提案帽 20：经第三方应用创建提案时，该用户 open 状态提案已达 20 条则返回 429。',
      '引用类校验发生在批准合并时：提案保存成功不等于能合入，审核者批准时可能得到 422。',
      '准入需要两步：应用经 user_login 自助申请 catalog:edit scope；应用的 client 还须由平台绑定目录租户（catalog_site）后本面才放行——目前为人工开通，请联系平台。'
    ]
  },
  {
    key: 'news',
    label: '资讯',
    name: '资讯 API（授权制）',
    file: NEWS_SPEC,
    prefix: '/v1/news',
    scope: () => 'news:read',
    auth: {
      kind: 'api_key',
      curl: 'Authorization: Bearer nm_live_<YOUR_KEY>',
      display: 'Authorization: Bearer nm_live_…',
      note: '机器 API 密钥,但须带 news:read —— 授权制 scope,登录门户后在控制台申请,批准后即可自助勾选'
    },
    notes: [
      '授权制：合作媒体授权给 NextMoe 的是一份索引，转授给谁由平台逐个决定，所以 news:read 不是自助勾一下就有的。在开发者门户控制台提交申请并说明用途，批准后这一项就出现在铸密钥的可勾选项里；没有它的密钥调这三条路径一律 403。',
      '这是索引，不是转载：每条只有标题、摘要与题图，正文既不下发也不留存。每一项都恒带来源块与 source_url，读者要看全文只能回到媒体自己的站点——渲染时必须把来源与链接一并展示。',
      '撤回即不可寻址：我们撤下的、以及上游原文已消失的条目会从列表中消失，按 id 直取则 404。这个 404 是契约而不是查询失败，不要重试，也不要拿缓存副本顶上。'
    ]
  },
  {
    key: 'v2',
    label: 'API v2',
    name: 'Public API v2（preview）',
    file: V2_SPEC,
    prefix: '/v2',
    scope: (_method, path) => {
      if (!path) return 'catalog:read'
      if (path.startsWith('/v2/me/') || path.startsWith('/v2/moderation/')) return ''
      if (
        path.startsWith('/v2/problems') ||
        path.startsWith('/v2/vocabularies') ||
        path.startsWith('/v2/news') ||
        path === '/v2/catalog/stats' ||
        path.startsWith('/v2/catalog/schemas/')
      ) {
        return ''
      }
      return 'catalog:read'
    },
    auth: {
      kind: 'api_key',
      curl: 'Authorization: Bearer nmk_live_<YOUR_KEY>',
      display: 'Authorization: Bearer nmk_live_…',
      note: 'v2 应用密钥。preview 期间只签发给 internal 档应用'
    },
    autoGroups: [
      { key: 'meta', label: '注册表', match: /^\/v2\/(problems|vocabularies)/ },
      { key: 'catalog', label: '目录', match: /^\/v2\/catalog/ },
      { key: 'news', label: '资讯', match: /^\/v2\/news/ },
      { key: 'me', label: '我的', match: /^\/v2\/me/ },
      { key: 'moderation', label: '审核', match: /^\/v2\/moderation/ }
    ],
    notes: [
      'preview：形状还可以改，包括删除与改名。第三方拿不到 /v2 凭证，继续用 /v1。',
      '/v2/me/playtimes 与 /v1/playtime 一样：用户令牌即可，不需要 playtime:read / playtime:write。任何已开通用户登录的应用都可以调用。',
      '错误体是 RFC 9457 application/problem+json。type URI 解析到本站 /problems/{domain}/{kebab-code}。',
      '客户端必须忽略未知字段、容忍开放词表中未见过的取值，并为未知错误 code 准备一个按 HTTP status 的兜底分支。'
    ]
  }
]

// Third-party tokens always 403 on these four; documenting them on the portal
// would imply they are callable. The completeness guard below requires every
// operation under /api/v1/user/catalog/edit to appear in include or here.
export const EDIT_EXCLUDED_OPERATION_IDS = [
  'amendEditProposalUser',
  'declineEditProposalUser',
  'mergeEditProposalUser',
  'revertEditEntityUser'
]

// Portal IA, keyed `METHOD path` so the two operations that share
// /v1/playtime/works/{workID} land in different groups. Membership is spelled
// out rather than matched by pattern: a guard below refuses to build when an
// operation belongs to no group AND when a listed operation no longer exists,
// so a spec that grows or renames a path fails the build instead of quietly
// dropping the operation into an "other" bucket nobody reads.
export const FACE_GROUPS = {
  catalog: [
    {
      key: 'discovery',
      label: '检索与反查',
      ops: [
        'GET /v1/catalog/search',
        'GET /v1/catalog/works/search',
        'GET /v1/catalog/lookup',
        'POST /v1/catalog/lookup/batch',
        'POST /v1/catalog/resolve',
        'GET /v1/catalog/redirects'
      ]
    },
    {
      key: 'works',
      label: '作品',
      ops: ['GET /v1/catalog/works', 'GET /v1/catalog/works/{id}']
    },
    {
      key: 'work-blocks',
      label: '作品子资源',
      ops: [
        'GET /v1/catalog/works/{id}/covers',
        'GET /v1/catalog/works/{id}/screenshots',
        'GET /v1/catalog/works/{id}/tags',
        'GET /v1/catalog/works/{id}/characters',
        'GET /v1/catalog/works/{id}/credits',
        'GET /v1/catalog/works/{id}/releases',
        'GET /v1/catalog/works/{id}/intros',
        'GET /v1/catalog/works/{id}/ratings',
        'GET /v1/catalog/works/{id}/relations',
        'GET /v1/catalog/works/{id}/series',
        'GET /v1/catalog/works/{id}/links',
        'GET /v1/catalog/works/{id}/engines'
      ]
    },
    {
      key: 'releases',
      label: '发售与日历',
      ops: [
        'GET /v1/catalog/releases',
        'GET /v1/catalog/calendar',
        'GET /v1/catalog/calendar/pending',
        'GET /v1/catalog/calendar/tba'
      ]
    },
    {
      key: 'people',
      label: '角色与人物',
      ops: ['GET /v1/catalog/characters/{id}', 'GET /v1/catalog/names/{id}']
    },
    {
      key: 'taxonomy',
      label: '厂牌与标签',
      ops: [
        'GET /v1/catalog/labels',
        'GET /v1/catalog/labels/{id}',
        'GET /v1/catalog/labels/{id}/relation-graph',
        'GET /v1/catalog/tags',
        'GET /v1/catalog/tags/{id}'
      ]
    },
    {
      key: 'series',
      label: '系列与引擎',
      ops: [
        'GET /v1/catalog/series',
        'GET /v1/catalog/series/{id}',
        'GET /v1/catalog/engines',
        'GET /v1/catalog/engines/{id}'
      ]
    },
    {
      key: 'sync',
      label: '变更流与统计',
      ops: ['GET /v1/catalog/changes', 'GET /v1/catalog/stats']
    }
  ],
  playtime: [
    {
      key: 'report',
      label: '上报',
      ops: [
        'PUT /v1/playtime/works/{workID}',
        'PUT /v1/playtime/by-ref/{source}/{externalID}',
        'POST /v1/playtime/batch'
      ]
    },
    {
      key: 'sync',
      label: '回拉',
      ops: ['GET /v1/playtime/mine', 'GET /v1/playtime/works/{workID}']
    }
  ],
  edit: [
    {
      key: 'schema',
      label: 'schema 与快照',
      ops: [
        'GET /api/v1/user/catalog/edit/schema/{entity_type}',
        'GET /api/v1/user/catalog/edit/snapshot'
      ]
    },
    {
      key: 'proposals',
      label: '提案',
      ops: [
        'POST /api/v1/user/catalog/edit/proposals',
        'GET /api/v1/user/catalog/edit/proposals',
        'GET /api/v1/user/catalog/edit/proposals/{id}',
        'POST /api/v1/user/catalog/edit/proposals/{id}/withdraw'
      ]
    }
  ],
  news: [
    {
      key: 'feed',
      label: '资讯读面',
      ops: ['GET /v1/news', 'GET /v1/news/sources', 'GET /v1/news/{id}']
    }
  ]
}

// The one operation whose credential differs from its face's. 2026-08-18 moved
// /v1/catalog/stats out of the key gate: it is the counter the public site and
// this portal's own landing page render before anyone holds a key.
export const NO_AUTH = {
  kind: 'none',
  curl: '',
  display: '无需凭据',
  note: '匿名可调 —— 目录规模是公开数字,不需要 API 密钥'
}
export const OPERATION_AUTH_OVERRIDES = { getCatalogStatsPublic: NO_AUTH }

export const USER_TOKEN_AUTH = {
  kind: 'user_token',
  curl: 'Authorization: Bearer <ACCESS_TOKEN>',
  display: 'Authorization: Bearer <用户访问令牌>',
  note: '用户授权后的访问令牌,不是 API 密钥'
}

export const EXPECTED_OPERATION_COUNTS = { catalog: 37, playtime: 5, edit: 6, news: 3, v2: 74 }
