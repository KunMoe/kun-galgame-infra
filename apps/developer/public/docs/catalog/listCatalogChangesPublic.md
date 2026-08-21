# Incremental works changes feed ((updated,id) keyset; next_cursor always present — keep polling it for new rows) · 目录数据 API（只读）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/catalog/changes

Incremental works changes feed ((updated,id) keyset; next_cursor always present — keep polling it for new rows)

Creations and updates of LIVE galgame works, ordered by (updated_at, id) ASC. The feed deliberately trails real time by ~5 seconds: updated_at is statement time, not commit time, so serving rows younger than that lag would let a slow transaction commit behind an already-advanced consumer cursor and be skipped forever. DELETIONS DO NOT FLOW THROUGH THIS FEED — a row that leaves the LIVE set simply stops appearing; merge-style disappearances are covered by /v1/catalog/redirects, and mirror-style consumers should periodically reconcile the full id set via works?sort=id. WATERMARK GUARANTEE: any write that changes what a work's public face renders — its own columns and every works?include= block (names, intros, labels, ratings, covers, refs) plus release_date — moves that work's updated_at, so a full mirror can be driven from this feed alone. The guarantee covers sub-resource and fan-out writes (a cover detached, a label logo cleared, an anchor confirmed or killed, a release edited, a merge rehanging facets onto the survivor), and it is one-directional: updated_at moving does not promise the rendered bytes differ, so consumers must diff, not trust.

- 所属 API：目录数据 API（只读）（/v1/catalog）
- 鉴权：Authorization: Bearer nm_live_…
- scope：catalog:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `entity_type` | query | 否 | string | v1 feed scope: work (default) 取值：work |
| `cursor` | query | 否 | string | Opaque keyset cursor; omit to start from the beginning |
| `limit` | query | 否 | integer (int64) | Items per page 1-500 (default 100); above 500 is clamped to 500, a non-positive or non-numeric value is a 400 |

```bash
curl "https://api.nextmoe.dev/v1/catalog/changes" \
  -H "Authorization: Bearer nm_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/catalog/listCatalogChangesPublic
