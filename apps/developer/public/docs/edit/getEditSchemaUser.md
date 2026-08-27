# Field schema + THIS TOKEN's evaluated field-level capabilities. Same projection as the S2S op, with no actor query parameters at all: a caller cannot ask what some other user would be allowed to do · 编辑提案 API

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /api/v1/user/catalog/edit/schema/{entity_type}

Field schema + THIS TOKEN's evaluated field-level capabilities. Same projection as the S2S op, with no actor query parameters at all: a caller cannot ask what some other user would be allowed to do

- 所属 API：编辑提案 API（/api/v1/user/catalog/edit）
- 鉴权：Authorization: Bearer <用户访问令牌>
- scope：catalog:edit

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `entity_type` | path | 是 | string | Registered entity type, e.g. catalog.work |
| `entity_id` | query | 否 | integer (int64) | Entity-aware projection subject (0 = type-level projection) |

```bash
curl "https://api.nextmoe.dev/api/v1/user/catalog/edit/schema/value" \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/edit/getEditSchemaUser
