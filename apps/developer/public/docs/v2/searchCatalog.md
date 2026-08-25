# Search catalog entities · Public API v2（preview）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v2/catalog/search

Search catalog entities

Cross-entity search. object= selects the family. Hits are search_result rows with target_object. Requires an application key. cursor= and ids= are not accepted.

- 所属 API：Public API v2（preview）（/v2）
- 鉴权：Authorization: Bearer nmk_live_…
- scope：catalog:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `q` | query | 否 | string | Search string. Empty runs a popularity-ordered listing of that family. |
| `object` | query | 否 | string | Required family: work, character, credit_name, company, tag. |
| `locale` | query | 否 | string | zh or ja. Ignored for works. Must not be used as a discriminant. |

```bash
curl "https://api.nextmoe.dev/v2/catalog/search" \
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/v2/searchCatalog
