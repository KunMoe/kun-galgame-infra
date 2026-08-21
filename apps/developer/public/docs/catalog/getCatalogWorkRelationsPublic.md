# Related works of one work, paged — the block works/{id} only serves under include=relations · 目录数据 API（只读）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/catalog/works/{id}/relations

Related works of one work, paged — the block works/{id} only serves under include=relations

ONE block of works/{id}, addressable on its own so a consumer that wants it does not pay for the other thirty keys — a data-rich work is 50 KB with include=relations,credits, of which the identity core is under a tenth. Items are the SAME objects the parent block carries: same schema, same order, same election, same suppression rules. There is deliberately no second "detail" shape for a sub-resource to drift into. VISIBILITY IS THE PARENT'S, VERBATIM — LIVE galgame works only, and a work works/{id} 404s 404s on every sub-resource too. PAGED with limit/offset (1-100, default 100); next_offset is present only while rows remain, and absent means the block is exhausted. The array embedded in works/{id} stays UNCAPPED: capping a published field is not a backward-compatible change, so the two faces differ in their bounds and in nothing else. With nsfw absent or 0 an r18 relation end is dropped WHOLE, not emptied, and next_offset counts what survived that drop. Sequels and fandiscs also reach you through works/{id}.series_siblings, which is the transitive series component rather than the one-hop relation set.

- 所属 API：目录数据 API（只读）（/v1/catalog）
- 鉴权：Authorization: Bearer nm_live_…
- scope：catalog:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `id` | path | 是 | integer (int64) | Catalog work id — the very id works/{id} answers on |
| `limit` | query | 否 | integer (int64) | Items per page 1-100 (default 100); above 100 is clamped to 100, a non-positive or non-numeric value is a 400 |
| `offset` | query | 否 | integer (int64) | Rows to skip |
| `nsfw` | query | 否 | boolean | true/1 = serve this sub-resource for an r18 work (default false = 404, exactly what works/{id} answers for the same work). It gates the WORK, not the rows: a cover's or screenshot's own sexual/violence level is reported, never filtered, and on the two blocks that carry a work_count that count is taken over the population this caller can actually fetch |

```bash
curl "https://api.nextmoe.dev/v1/catalog/works/1/relations" \
  -H "Authorization: Bearer nm_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/catalog/getCatalogWorkRelationsPublic
