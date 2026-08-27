# 编辑提案 API

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

- 路径前缀：`/api/v1/user/catalog/edit`
- 凭据：Authorization: Bearer <用户访问令牌>（用户授权后的访问令牌(OAuth 授权码 + PKCE),需 catalog:edit scope）
- 端点数：6

## 使用须知

- 第三方应用的令牌恒为「只提案」姿态：审核、自动合入、撤销他人提案永不可；schema 投影对它 can_review=false。
- 每用户未决提案帽 20：经第三方应用创建提案时，该用户 open 状态提案已达 20 条则返回 429。
- 引用类校验发生在批准合并时：提案保存成功不等于能合入，审核者批准时可能得到 422。
- 准入需要两步：应用经 user_login 自助申请 catalog:edit scope；应用的 client 还须由平台绑定目录租户（catalog_site）后本面才放行——目前为人工开通，请联系平台。

## 端点

### schema 与快照

- `GET /api/v1/user/catalog/edit/schema/{entity_type}` — Field schema + THIS TOKEN's evaluated field-level capabilities. Same projection as the S2S op, with no actor query parameters at all: a caller cannot ask what some other user would be allowed to do [详情](https://developer.nextmoe.dev/docs/edit/getEditSchemaUser.md)
- `GET /api/v1/user/catalog/edit/snapshot` — The entity's current registered-field values (the editor's bootstrap read), same shape as the S2S op. Authenticated but NOT tenant-fenced: it projects the same entity state the public reads already render [详情](https://developer.nextmoe.dev/docs/edit/getEditSnapshotUser.md)

### 提案

- `POST /api/v1/user/catalog/edit/proposals` — File an edit proposal AS THE BEARER TOKEN'S OWN USER (automerges into a direct edit when the caller's token roles already carry the review capability, and NEVER when the token was issued through a third-party application). The proposer and the filing tenant are derived from the token; the body carries neither [详情](https://developer.nextmoe.dev/docs/edit/createEditProposalUser.md)
- `GET /api/v1/user/catalog/edit/proposals` — List edit proposals on the token client's catalog site. mine=true is the token user's OWN filing history (no permission needed); mine absent is the REVIEW QUEUE and requires the same review authority the merge/decline ops need for that entity_type (403 otherwise). Neither site nor proposer_uid is a parameter [详情](https://developer.nextmoe.dev/docs/edit/listEditProposalsUser.md)
- `GET /api/v1/user/catalog/edit/proposals/{id}` — Read one proposal with its amendments and effective patch (same shape as the S2S detail read); refuses a proposal filed on another tenant [详情](https://developer.nextmoe.dev/docs/edit/getEditProposalUser.md)
- `POST /api/v1/user/catalog/edit/proposals/{id}/withdraw` — Withdraw one's OWN open proposal. Bodiless: the only identity involved is the token's, and the engine refuses any proposal the token's user did not file [详情](https://developer.nextmoe.dev/docs/edit/withdrawEditProposalUser.md)

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/edit
