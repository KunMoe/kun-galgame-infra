# Reverse-lookup an external id via an EXACT anchor (killer feature); type=work|name|character|label, 404 on miss/hidden · 目录数据 API（只读）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/catalog/lookup

Reverse-lookup an external id via an EXACT anchor (killer feature); type=work|name|character|label, 404 on miss/hidden

- 所属 API：目录数据 API（只读）（/v1/catalog）
- 鉴权：Authorization: Bearer nmk_live_…
- scope：catalog:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `source` | query | 否 | string | Upstream source key: vndb \| bangumi \| dlsite \| erogamescape (the legacy misspelling erogamespace is still accepted) |
| `external_id` | query | 否 | string | The id within that source. type=work accepts vndb v19658 or 19658 (and dlsite RJ/VJ numbers); the non-work types match VERBATIM as the registry stores them — vndb character c1234, label p129, staff a bare number |
| `type` | query | 否 | string | Which entity family the external id is resolved against (default work); an unknown token is a 400, whereas an unknown SOURCE is a miss 取值：work \| name \| character \| label |
| `nsfw` | query | 否 | boolean | true/1 = resolve r18 works too (default false = 404 on an r18 hit); on type=character it also keeps sexual traits |

```bash
curl "https://api.nextmoe.dev/v1/catalog/lookup" \
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/catalog/lookupCatalogPublic
