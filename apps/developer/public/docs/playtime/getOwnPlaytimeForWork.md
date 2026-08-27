# The bearer token's own playtime on ONE work, folded across their applications (MAX minutes — two apps watching one save file are not two playthroughs). `playtime` is null when the user has never reported here; that is a 200, not a 404. This is the call a rating form makes to offer 'you played 30h — attach it?'. Any app with a user access token may call this; playtime:read is not required. · 游玩时长 API

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/playtime/works/{workID}

The bearer token's own playtime on ONE work, folded across their applications (MAX minutes — two apps watching one save file are not two playthroughs). `playtime` is null when the user has never reported here; that is a 200, not a 404. This is the call a rating form makes to offer 'you played 30h — attach it?'. Any app with a user access token may call this; playtime:read is not required.

- 所属 API：游玩时长 API（/v1/playtime）
- 鉴权：Authorization: Bearer <用户访问令牌>
- scope：无需凭据

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `workID` | path | 是 | integer (int64) | Catalog work id |

```bash
curl "https://api.nextmoe.dev/v1/playtime/works/1" \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/playtime/getOwnPlaytimeForWork
