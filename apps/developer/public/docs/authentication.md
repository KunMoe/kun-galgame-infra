# 鉴权与凭据

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

一条请求只带一个凭据。带哪一个，由你读的是「目录里的公共事实」还是「某个用户自己的东西」决定。

## 三种身份

| 身份         | 请求头                                 | 代表谁             | 用在哪些前缀                                                                                        |
| ------------ | -------------------------------------- | ------------------ | --------------------------------------------------------------------------------------------------- |
| 应用密钥     | `Authorization: Bearer nmk_live_…`     | 你的应用           | `/v2/catalog`、`/v2/store`                                                                          |
| 用户访问令牌 | `Authorization: Bearer <access_token>` | 授权给你的那个用户 | `/v2/me`、`/v2/moderation`                                                                          |
| 匿名         | 不带                                   | 任何人             | `/v2/news`、`/v2/vocabularies`、`/v2/problems`、`/v2/catalog/stats`、`/v2/catalog/schemas/{object}` |

> [!NOTE]
> 一条请求只带一个凭据。如果某个操作看起来需要两种身份同时在场，那它是被放错了面——请告诉我们，而不是想办法同时塞两个。

## 应用密钥

- 前缀 `nmk_live_`（生产）或 `nmk_test_`（开发联调），尾部带 CRC32 校验位——手抖改错一个字符能在到达服务端前就被认出来。
- **只在铸造时显示一次**。丢了就吊销重铸，没有找回。
- 它是机密：只放服务端。浏览器、移动端二进制、公开仓库、CI 日志都不行。前端要用数据，请让自己的后端代理。
- 每个应用最多 5 把在用密钥——轮换时先铸新的、灰度切流、再吊销旧的，不必停机。

### scope

| scope               | 开什么                                                                          | 怎么拿                   |
| ------------------- | ------------------------------------------------------------------------------- | ------------------------ |
| `catalog:read`      | 整个 `/v2/catalog` 只读面                                                       | 控制台自助勾选           |
| `store:read`        | `/v2/store` 商店联盟链接与统计                                                  | 控制台自助勾选           |
| `claim_events:read` | 认领事件 feed 与自家站点的审核队列状态（`claim_state=pending,declined,hidden`） | 运营方按需授予，不能自助 |

`claim_events:read` 不开放自助是有原因的：那条 feed 里带着每次拒绝的理由和做出决定的审核员 uid。

## 用户访问令牌

`/v2/me` 与 `/v2/moderation` 读写的是**某个用户自己的东西**——他的游玩时长、他提交的编辑提案、他的认领、他投的封面票。应用密钥在这两个前缀下一律无效，因为它证明不了「哪个用户」。

拿令牌走标准的 OAuth 2.0 授权码 + PKCE：

1. 把用户跳到 `https://oauth.kungal.com/api/v1/oauth/authorize`，带上 `response_type=code`、`client_id`、`redirect_uri`、`scope`、`state` 与 `code_challenge` / `code_challenge_method=S256`。
2. 用户同意后回调你的 `redirect_uri`，带回 `code`。校验 `state`。
3. `POST https://oauth.kungal.com/api/v1/oauth/token`，用 `code` + `code_verifier` 换 `access_token`（JWT，15 分钟）与 `refresh_token`。
4. 带 `access_token` 调 `/v2/me/*`；过期后用 `refresh_token` 刷新，每次刷新都会轮换。

> [!WARNING]
> `refresh_token` 是**不透明随机串**，不是 JWT。不要解析它、不要从里面读过期时间——过期的唯一信号是刷新失败。

OAuth 端点的线格式是裸 RFC 6749（`{access_token, token_type, expires_in, …}`），同样没有信封；失败是 `{"error": "...", "error_description": "..."}`。完整契约见统一文档门户 `docs-kungal.nextmoe.dev`。

## 认证失败长什么样

| 状态 | `code`                   | 含义                               | 怎么办                        |
| ---- | ------------------------ | ---------------------------------- | ----------------------------- |
| 401  | `MISSING_CREDENTIAL`     | 没有 `Authorization` 头            | 带上密钥                      |
| 401  | `INVALID_CREDENTIAL`     | 凭据存在，但无效、过期或已被吊销   | 换一把密钥；用户令牌则去刷新  |
| 403  | `SCOPE_REQUIRED`         | 凭据有效，但缺这个操作要的 scope   | 在控制台补勾 scope 并重铸密钥 |
| 403  | `USER_IDENTITY_REQUIRED` | 这个面要用户身份，你带的是应用密钥 | 改用用户访问令牌              |

错误体是 RFC 9457 `application/problem+json`，字段与分支写法见 [错误处理](/docs/errors)。

## 限流身份跟着凭据走

密钥按**密钥所属应用**计数，用户令牌按**用户**计数，匿名按 IP 计数。也就是说，把整个用户群从一个服务端出口代理出去时，请用用户令牌调用用户面——否则所有人会挤进同一个匿名 IP 桶。详见 [限流与配额](/docs/rate-limits)。

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/authentication
