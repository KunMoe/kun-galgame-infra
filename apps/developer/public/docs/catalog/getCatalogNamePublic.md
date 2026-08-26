# Credited identity (same-person grouping via public links); include=credits attaches works + roles · 目录数据 API（只读）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/catalog/names/{id}

Credited identity (same-person grouping via public links); include=credits attaches works + roles

- 所属 API：目录数据 API（只读）（/v1/catalog）
- 鉴权：Authorization: Bearer nmk_live_…
- scope：catalog:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `id` | path | 是 | integer (int64) | Credit-name id (the addressable credited identity) |
| `include` | query | 否 | string | credits = attach the works this name is credited on |
| `nsfw` | query | 否 | boolean | true/1 = include r18 works among the credits (default false = dropped) |
| `limit` | query | 否 | integer (int64) | Credits per page 1-50 (default 50); above 50 is clamped to 50, a non-positive or non-numeric value is a 400 |
| `offset` | query | 否 | integer (int64) | Rows to skip |

```bash
curl "https://api.nextmoe.dev/v1/catalog/names/1" \
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/catalog/getCatalogNamePublic
