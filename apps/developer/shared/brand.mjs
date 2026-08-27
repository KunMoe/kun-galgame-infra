// Plain .mjs on purpose: both the Vue pages and scripts/sync-specs.mjs (bare
// node, no TS loader) import this, so the attribution wording has exactly one
// copy and the rendered page can never drift from llms.txt.

export const ATTRIBUTION_NOTE =
  '目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。'

export const ATTRIBUTION_NOTE_EN =
  'While this stage of the NextMoe API is in use, credit the API as 鲲 Galgame 论坛 (KUN Galgame forum) if you use galgame data, or LetMoe·一启萌 if you use doujin-game data.'

export const SITE_URL = 'https://developer.nextmoe.dev'
export const API_HOST = 'https://api.nextmoe.dev'
export const MCP_ENDPOINT = 'https://mcp.nextmoe.dev/mcp'

export const SOURCES = [
  ['VNDB', '身份主锚、关系、角色 traits'],
  ['Bangumi', '中文名与条目、角色资料'],
  ['DLsite', '同人与商业店铺条目'],
  ['ErogameScape', '评分与发售信息'],
  ['Ci-en', '创作者动态与厂牌外链'],
  ['Getchu', '角色立绘、正文、截图']
]
