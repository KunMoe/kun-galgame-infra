# Report the bearer token's own playtime on a work. The body carries the ABSOLUTE cumulative total in minutes, never a delta — re-sending the same number is a no-op, which makes the call safe to retry. Keyed by (user, work, client): a second app of the same user reports alongside, not over. Any app with a user access token may call this; playtime:write is not required. · 游玩时长 API

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## PUT /v1/playtime/works/{workID}

Report the bearer token's own playtime on a work. The body carries the ABSOLUTE cumulative total in minutes, never a delta — re-sending the same number is a no-op, which makes the call safe to retry. Keyed by (user, work, client): a second app of the same user reports alongside, not over. Any app with a user access token may call this; playtime:write is not required.

- 所属 API：游玩时长 API（/v1/playtime）
- 鉴权：Authorization: Bearer <用户访问令牌>
- scope：无需凭据

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `workID` | path | 是 | integer (int64) | Catalog work id |

```bash
curl -X PUT "https://api.nextmoe.dev/v1/playtime/works/1" \
  -H "Authorization: Bearer <ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"minutes":0}'
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/playtime/reportPlaytime
