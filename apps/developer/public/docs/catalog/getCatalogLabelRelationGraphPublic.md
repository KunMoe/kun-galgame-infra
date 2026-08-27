# Corporate-structure graph around a label: the connected family (parents, subsidiaries, imprints, spin-offs, succession) in one call · 目录数据 API（只读）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/catalog/labels/{id}/relation-graph

Corporate-structure graph around a label: the connected family (parents, subsidiaries, imprints, spin-offs, succession) in one call

labels/{id}.relations[] is ONE HOP; this is the whole component, which is what a picture needs — standing on a brand you can see its parent's other brands without walking the one-hop face once per neighbour. A breadth-first walk from the seed over the label-relation graph, bounded at depth 4 and 60 nodes; the bound is applied breadth-first, so a truncated answer keeps the neighbourhood NEAREST the seed. Soft-deleted (merged-away) labels never appear. There is no pagination — a graph served in slices is not a graph. nodes[0] is always the seed, and a label with no relations is a one-node, zero-edge graph, not a 404; an unknown id is a 404 and a merged id is the same 301 labels/{id} serves. EDGE SEMANTICS: {from, to, relation} reads "`to` is the `relation` of `from`" — the same reading relations[].relation has, where `from` is the label being viewed. So {from: Key, to: VisualArt's, relation: parent} means "VisualArt's is the parent of Key". The underlying graph is stored MIRRORED, but each fact is emitted ONCE: only the canonical side of each inverse pair (parent, imprint, spawned, succeeded_by) is rendered, and the four inverses (subsidiary, imprint_of, origin, formerly) are implied by reading the edge backwards — for "the subsidiaries of X", take the edges whose `to` is X and whose relation is parent. Every node's work_count is the SAME nsfw-aware number labels/{id} and the labels browse lane report for this caller.

- 所属 API：目录数据 API（只读）（/v1/catalog）
- 鉴权：Authorization: Bearer nmk_live_…
- scope：catalog:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `id` | path | 是 | integer (int64) | Catalog label id — the seed the graph is grown from |
| `nsfw` | query | 否 | boolean | true/1 = count r18 works in every node's work_count (default false = excluded, matching an sfw labels/{id} call) |

```bash
curl "https://api.nextmoe.dev/v1/catalog/labels/1/relation-graph" \
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/catalog/getCatalogLabelRelationGraphPublic
