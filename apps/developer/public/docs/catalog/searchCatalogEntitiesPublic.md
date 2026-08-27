# Entity autocomplete over names / characters / labels / works / tags, projected to public briefs · 目录数据 API（只读）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/catalog/search

Entity autocomplete over names / characters / labels / works / tags, projected to public briefs

The identity finder: up to 20 flat hits of ONE family, no filters and no pagination — what a picker or a jump-to-entity box needs. For a works RESULTS PAGE (filters, facets, sort, paging, full works-list rows) use GET /v1/catalog/works/search instead.

- 所属 API：目录数据 API（只读）（/v1/catalog）
- 鉴权：Authorization: Bearer nmk_live_…
- scope：catalog:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `type` | query | 否 | string | Which index to search; works (wave 105) searches every LIVE galgame registry work by any title, tags (A2-1d) the canonical cross-source tag vocabulary 取值：names \| characters \| labels \| works \| tags |
| `q` | query | 否 | string | Search text; empty returns the most-credited entities |
| `locale` | query | 否 | string | UI locale; the server pins the query language 取值：zh \| ja \| en |
| `limit` | query | 否 | integer (int64) | Max hits (capped at 20) |
| `nsfw` | query | 否 | boolean | works only: true/1 = include r18 hits (default false = excluded server-side) |

```bash
curl "https://api.nextmoe.dev/v1/catalog/search" \
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/catalog/searchCatalogEntitiesPublic
