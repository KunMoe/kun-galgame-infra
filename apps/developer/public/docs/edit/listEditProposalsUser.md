# List edit proposals on the token client's catalog site. mine=true is the token user's OWN filing history (no permission needed); mine absent is the REVIEW QUEUE and requires the same review authority the merge/decline ops need for that entity_type (403 otherwise). Neither site nor proposer_uid is a parameter · 编辑提案 API

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /api/v1/user/catalog/edit/proposals

List edit proposals on the token client's catalog site. mine=true is the token user's OWN filing history (no permission needed); mine absent is the REVIEW QUEUE and requires the same review authority the merge/decline ops need for that entity_type (403 otherwise). Neither site nor proposer_uid is a parameter

- 所属 API：编辑提案 API（/api/v1/user/catalog/edit）
- 鉴权：Authorization: Bearer <用户访问令牌>
- scope：catalog:edit

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `entity_type` | query | 否 | string | Entity type to list (required: authority is resolved per type) |
| `entity_id` | query | 否 | integer (int64) | Narrow to one entity; 0 = the whole type |
| `status` | query | 否 | string | Filter by status; empty = all 取值： \| open \| merged \| declined \| withdrawn |
| `limit` | query | 否 | integer (int64) | Page size (max 200, default 50) |
| `mine` | query | 否 | boolean | true = only the token user's own proposals (no review permission needed); false/absent = the review queue for this entity type, which requires review authority |

```bash
curl "https://api.nextmoe.dev/api/v1/user/catalog/edit/proposals" \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/edit/listEditProposalsUser
