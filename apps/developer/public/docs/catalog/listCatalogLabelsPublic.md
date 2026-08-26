# Keyset label browse lane (id ASC); filter by kind, each row carries an nsfw-aware work_count · 目录数据 API（只读）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/catalog/labels

Keyset label browse lane (id ASC); filter by kind, each row carries an nsfw-aware work_count

Every label that has not been merged away, id ascending. work_count is the number of works THIS caller would page through via works?label_id=<id> — so an sfw caller's count excludes r18 works and always matches the list it can actually fetch.

- 所属 API：目录数据 API（只读）（/v1/catalog）
- 鉴权：Authorization: Bearer nmk_live_…
- scope：catalog:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `kind` | query | 否 | string | Filter by label kind; a token outside this closed set is a 400 取值：game_brand \| bunko \| publisher \| anime_studio \| doujin_circle \| group |
| `cursor` | query | 否 | string | Opaque keyset cursor from a prior next_cursor; omit for the first page |
| `limit` | query | 否 | integer (int64) | Items per page 1-100 (default 20); above 100 is clamped to 100, a non-positive or non-numeric value is a 400 |
| `nsfw` | query | 否 | boolean | true/1 = count r18 works in work_count (default false = excluded, matching what an sfw works?label_id= call returns) |
| `has_works` | query | 否 | boolean | true/1 = only labels whose work_count is > 0 under the same nsfw setting (default false = every label); total converges with the filter |

```bash
curl "https://api.nextmoe.dev/v1/catalog/labels" \
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/catalog/listCatalogLabelsPublic
