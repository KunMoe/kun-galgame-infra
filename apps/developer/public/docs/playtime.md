# 游玩时长 API

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

- 路径前缀：`/v1/playtime`
- 凭据：Authorization: Bearer <用户访问令牌>（用户访问令牌。任何已开通用户登录的应用都可以调用，不需要 playtime:read / playtime:write）
- 端点数：5

## 端点

### 上报

- `PUT /v1/playtime/works/{workID}` — Report the bearer token's own playtime on a work. The body carries the ABSOLUTE cumulative total in minutes, never a delta — re-sending the same number is a no-op, which makes the call safe to retry. Keyed by (user, work, client): a second app of the same user reports alongside, not over. Any app with a user access token may call this; playtime:write is not required. [详情](https://developer.nextmoe.dev/docs/playtime/reportPlaytime.md)
- `PUT /v1/playtime/by-ref/{source}/{externalID}` — Report playtime addressing the work by an external id the client already holds (vndb/dlsite/getchu/bangumi …) instead of a catalog work id. Only EXACT anchors resolve; the response echoes the resolved work_id, which the client should cache. 404 when nothing is anchored to that id. Any app with a user access token may call this; playtime:write is not required. [详情](https://developer.nextmoe.dev/docs/playtime/reportPlaytimeByRef.md)
- `POST /v1/playtime/batch` — Report up to 200 works in one call — the first-login library sync. Each item is accepted or rejected on its own and the response reports per-item outcomes; a single bad item never fails the batch. Any app with a user access token may call this; playtime:write is not required. [详情](https://developer.nextmoe.dev/docs/playtime/reportPlaytimeBatch.md)

### 回拉

- `GET /v1/playtime/mine` — Page the bearer token's own playtime rows in (updated_at) order — the sync-back leg for a second device. Hand `cursor` back as ?updated_since= to fetch only what changed. Any app with a user access token may call this; playtime:read is not required. [详情](https://developer.nextmoe.dev/docs/playtime/listOwnPlaytime.md)
- `GET /v1/playtime/works/{workID}` — The bearer token's own playtime on ONE work, folded across their applications (MAX minutes — two apps watching one save file are not two playthroughs). `playtime` is null when the user has never reported here; that is a 200, not a 404. This is the call a rating form makes to offer 'you played 30h — attach it?'. Any app with a user access token may call this; playtime:read is not required. [详情](https://developer.nextmoe.dev/docs/playtime/getOwnPlaytimeForWork.md)

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/playtime
