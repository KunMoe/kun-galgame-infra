# Claim event history · Public API v2

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v2/catalog/claim-events

Claim event history

Every claim lifecycle transition, newest first by default. sort=recorded_asc walks the same collection oldest-first by id, which is the shape a mirror or a reward cron reads with a watermark. Requires an application key with the claim_events:read scope on top of catalog:read; the scope is granted by an operator, not self-service, because events carry decline reasons and moderator actions.

- 所属 API：Public API v2（/v2）
- 鉴权：Authorization: Bearer nmk_live_…
- scope：catalog:read + claim_events:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `cursor` | query | 否 | string | Opaque keyset cursor from a prior next_cursor. Must start with cur_. |
| `limit` | query | 否 | string | Page size 1-100, default 20. Values above 100 are 400 LIMIT_TOO_LARGE, not clamped. |
| `view` | query | 否 | string | basic (default) or full. Closed vocabulary. |
| `fields` | query | 否 | string | Comma-separated top-level keys. Unknown token is 400 UNKNOWN_FIELD. object and id are always kept. |
| `ids` | query | 否 | string | Comma-separated event ids, max 100. Batch lane: no pagination. |
| `include_total` | query | 否 | string | true to include total. Only true or false. |
| `sort` | query | 否 | string | recorded_desc (default) or recorded_asc. Closed. recorded_asc is the watermark walk a mirror reads. |
| `site` | query | 否 | string | Tenant key the event was recorded under. Open vocabulary; unknown values match nothing. |
| `actor_uid` | query | 否 | string | The claiming site's own user id of the actor. Not a catalog id. |
| `work_id` | query | 否 | string | Catalog work id to narrow to one work's claim history. |

```bash
curl "https://api.nextmoe.dev/v2/catalog/claim-events" \
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/v2/listCatalogClaimEvents
