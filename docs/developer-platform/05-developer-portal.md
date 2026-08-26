# 开发者门户

> 本文承载 §9 开发者门户(`developer.nextmoe.dev`)。设计与命名约定见 [01-design.md](./01-design.md);门户展示的公开 spec 与 OpenAPI 策略见 [02 §10](./02-public-api.md)。

---

## 9. 开发者门户(`developer.nextmoe.dev`)

- **定名与定位(2026-07-28,用户裁定)**:名字朴素——**NextMoe 开发者平台**;定位语 = **「ACGN 数据,以此为准」**(当各源各执一词,NextMoe 逐字段裁定唯一标准答案;首发 Galgame 面,同构扩展至全部 ACGN 媒介)。API 面名不变(spec title 属冻结契约不碰)。
- **账号复用**:用生态账号经 IdP 登录即开发者账号,**不另造身份**(品牌显示随「NextMoe 账户」改名同步,机制零变)。
- **核心功能**:
  1. 创建应用(= 一行 `oauth_clients`,`owner_user_id=当前用户`,`dev_enabled=true`)→ 拿 `client_id`(OAuth)。
  2. 管理 **API Keys**:创建(**show-once** 明文)、看 `prefix+last4+last_used`、轮换(带宽限)、吊销。
  3. **用量/配额**(`/usage` 页,读 `GET /dev/usage?days=N`,窗口 7/14/30 天):
     - **每日调用量**柱状图 + 窗口合计(总请求 / 错误率 / 4xx / 5xx)——读 `developer_api_usage` rollup 的稠密日序列。
     - **按应用** / **按面** 两张分解表(各按量降序)。
     - **实时配额剩余**:每把 active key 一张卡,显示今日剩余 / 每日配额 + 用量条 + 速率上限——**直接读 Redis 执法计数器**(与限流同源,非 rollup 估算)。计数后端不可达时该区降级为「暂不可用」提示,页面其余照常(`live_unavailable`)。
  4. **OpenAPI 文档**:用 **Scalar** 渲染(MIT、Try-It 最强、支持 OAuth flow、可嵌 Nuxt);两份公开 spec(catalog 面 / galgame 面)分 tab 呈现,未来媒介面同构加 tab。
  5. 申请更高 tier(走审批)。
- **技术**:门户前端 Nuxt(`apps/` 下新增或并入现有);平台后端扩展 account/IdP 侧的 API(应用/key/用量 CRUD,鉴权用现有 JWT + `owner_user_id` 归属校验)。

### 9.1 登录升级为 OP 跳转 SSO(拍板 2026-07-23 · 生产部署收官 2026-07-26)

门户登录已从本地密码表单升级为 **OAuth Authorization Code + PKCE(S256)跳转登录**(下游站点那套「已在 hub 登录即一键进入」)。门户即 IdP 的一个**第一方 confidential client**;OAuth token 落进**现有** access_token / refresh cookie 约定,`/dev/*` 与 `/auth/me` 靠同一 signer 的 access_token 直接消费(后端**零代码改动**)。

**实现(apps/developer,全部 client/Nitro 侧)**:

- `app/utils/oauth-pkce.ts` — PKCE code_verifier / code_challenge(S256)/ state 生成;
- `app/composables/useOAuthLogin.ts` — `startLogin` / `startRegister`:存 verifier+state+redirect 进 sessionStorage → 顶层跳 `{authorizeBase}/oauth/authorize`(register 先经 OP `/auth/register?redirect=`);
- `app/pages/auth/callback.vue` — 校验 state → POST `/auth/exchange` → 播种 access_token + 拉 user → 跳 redirect/`/dashboard`;
- `server/routes/auth/{exchange,refresh,logout}.post.ts` + `server/utils/oauth-session.ts` — 服务端换码/刷新/吊销 + 落 cookie(access_token JS 可读 Path=/、refresh_token httpOnly Path=/auth、auth_mode 标记);
- 登录 modal(`components/login/Modal.vue`)= SSO 主按钮 + 密码表单回退 + SSO 注册引导。

**关键契约发现(修正原「落进现有 refresh 约定」的假设)**:第一方 `/api/v1/auth/refresh` **拒绝 client-bound(OAuth)session**(`auth_service.go:611`)——OAuth session **只能**经 `/oauth/token` `grant_type=refresh_token` 刷新(轮换)。故门户用 Nitro `/auth/refresh` 包 `/oauth/token`;`auth_mode` cookie 选择刷新/登出路径(`oauth`→Nitro 路由、`password`→第一方 relay),密码回退路径完全不变。access_token 仍是同一 signer,`/dev/*`、`/auth/me` 零改动消费。

**部署配置(✅ 已于 2026-07-26 全部落地:client 注册 + 双 env + oauth/portal 部署;SSO 全链与 /dev/* 栅栏放行均经生产实测)**:

1. **注册 OAuth client**(admin,`POST /api/v1/oauth/clients`,或管理台):`redirect_uris=["https://developer.nextmoe.dev/auth/callback"]`、`grants=["authorization_code","refresh_token"]`、`is_public=false`(confidential,门户有 Nitro 服务端)、`auto_consent=true`(第一方跳过同意页)、`allowed_scopes=[]`(默认 openid/profile/email)。响应给出 `client_id` + 一次性明文 `client_secret`。
2. **配置门户环境变量**(生产):`NUXT_PUBLIC_OAUTH_CLIENT_ID`、`NUXT_OAUTH_CLIENT_SECRET`(服务端)、`NUXT_PUBLIC_OAUTH_AUTHORIZE_BASE=https://oauth.kungal.com/api/v1`、`NUXT_PUBLIC_OAUTH_WEB_BASE=https://oauth.kungal.com`、`NUXT_PUBLIC_OAUTH_REDIRECT_URI=https://developer.nextmoe.dev/auth/callback`。`redirect_uri` **完全串匹配**,勿有尾斜杠漂移。**注意:Dokploy Environment 面板的值只做 compose 变量替换——变量必须同时在 `docker-compose.developer.yml` 的 `environment:` 块里声明转发才会进入容器**(缺声明会静默回落到镜像构建期的 localhost 默认值,2026-07-23 首次部署实爆);五个 SSO 变量已全部声明,新增 runtime-config 键时须同步补这里。
3. **本地 dev**:向本地 `kun_galgame_infra` 播一条等效 client 行(redirect_uri 用 `http://127.0.0.1:9430/auth/callback`),并设对应 `NUXT_PUBLIC_OAUTH_*` / `NUXT_OAUTH_CLIENT_SECRET`;authorize/web base 指向本地 OP(API :9277 / 前端 :9420)。注意 `refresh-dev-db` 会抹掉一切手播的 dev-only client 行——刷新后需重播,或改用快照自带的 prod client + `dev-secret-<client_id>` 契约。**已固化配方(2026-07-26 首播)**:client 行 = `devportal-dev` / secret = `sha256:` + hex(sha256(`dev-secret-devportal-dev`))(公开 dev 凭证契约)/ confidential / `auto_consent=true` / grants `["authorization_code","refresh_token"]` / scopes `["openid","profile","email"]` / redirect 精确 `http://127.0.0.1:9430/auth/callback`,SQL 模板沿用 `docs/dev-environment.md` 的 letmoe-dev upsert(替换 VALUES 即可);门户侧 env 落 `apps/developer/.env`(gitignored)= 五个 SSO 变量 + **`NUXT_OAUTH_API_BASE=http://127.0.0.1:9277`**(nuxt.config 的 dev 默认是 `:19277`,本地 oauth 实际监听 `:9277`,不覆写则 Nitro 换码/刷新打不通)。**栅栏联动**:oauth 进程须带 `KUN_DEV_PORTAL_CLIENT_IDS=devportal-dev`,否则 SSO 登录成功但 `/dev/*` 被 DevPortalFence 403(fail-closed;密码回退不受影响)。**本地已固化为默认(2026-07-26)**:`apps/api/.env`(godotenv——air 热组与手起二进制皆读)、`apps/api/.env.example`、`docker-compose.dev.yml` oauth 块三处均为 `devportal-dev`;oauth 重启后协议级 E2E 全链绿(login → consent 签码 → 门户换码 → client-bound token → `/dev/*` 200 → refresh 轮换 → 再 200)。注意:`GET /oauth/authorize` 设计上恒 303 到 OP 前端页,授权码由 OP 前端 auto_consent 后打的 `POST /oauth/authorize/consent` 签发——脚本化 E2E 必须走 consent 腿。

> `/dev/*` 的 owner 判定按 uid、与 token 的 client 归属无关(已核:OAuth access_token 在 `/auth/me` 与 devapi 链上等价于直登 token)。

**验收后记(2026-07-23 双维度评审后修正)**:

- **token 读取器**(`server/utils/oauth-session.ts` `tokenWirePayload/tokenWireError`):`/oauth/token` 是 OAuth 协议端点,只有 RFC 6749 裸 shape(成功 `{access_token,...}` / 失败 `{error,error_description}`)。exchange/refresh 路由以 **access_token 存在性**判成败,不看任何状态字段——2026-07-25 线格式切换那天,只看 `code` 的读取器会静默全断,这条判据是唯一没被咬到的原因。
- **登出双模全清**(`useAuth.logout`):密码与 SSO 两种 session 的 refresh_token 同名不同 Path(`/api/v1/auth` vs `/auth`),登出无条件两路都打——只清当前 auth_mode 会让另一模式的存活 cookie 在下次导航时把用户「静默复活」登录(跨账号时更是错账号复活)。
- **瞬时刷新失败不清会话**(`useTokenRefresh` 返回 `REFRESH_TRANSIENT`):网络抖动 / IdP 5xx / Nitro 刷新路由的蓄意 503 不再被判成 session 死亡强制登出;仅 4xx(无 cookie / 过期 / 吊销)才清会话跳登录。**重试 UI(2026-07-26 补齐)**:transient 态现有全局呈现——`useRefreshTransient`(useState,记账收敛在单飞 promise 上)驱动 `layout/RefreshBanner` 固定横幅(说明会话仍有效 + 一键重试/忽略;以 `auth_mode` cookie 为「确有会话」门,匿名访客不见横幅);重试成功即落 token、拉 user、`refreshNuxtData()` 重取降级页面的数据,重试发现 session 已死才清态跳 /login。同波修正 `middleware/auth.ts`:原先把 transient 布尔坍缩成「未刷新→弹 /login」,违反本条契约;现改为三态——成功落 token 放行、transient 放行(页面降级渲染 + 横幅接手)、确死才弹登录。
- **已接受的偏差(有意为之,评审记录在案)**:access_token 为 JS 可读 cookie(沿袭 apps/web 约定;refresh_token httpOnly 兜底持久层);PKCE verifier/state 存 sessionStorage(confidential client 下 PKCE 是纵深防御,主认证在服务端 client_secret)。
- **client 栅栏(已拍板并实现,2026-07-23 wave 08)**:上面「owner 判定与 token 的 client 归属无关」原是隐患——`middleware.Auth` 只验 signer + uid、不查 token 属哪个 OAuth client,任何第三方 app 的用户 token(仅授 `openid profile email`)都能替用户铸/轮换/吊销 API key(confused-deputy)。已加 **`DevPortalFence`**(`middleware.Auth` 之后):第一方 `/auth/login` session token(`client_id==""`)与 env `KUN_DEV_PORTAL_CLIENT_IDS` 白名单内的 client 放行,其余 403;**空白名单 = fail-closed(仅放行第一方)**。因此门户专属 client 注册后,须把它的 `client_id` 填进 **oauth 服务**的 `KUN_DEV_PORTAL_CLIENT_IDS`,否则门户自己的 SSO 用户会被栅栏 403(密码回退不受影响)。完整契约与 `dev:manage` 升级路径见 [03 §4.4](./03-auth-and-tiers.md)。

### 9.2 应用自助登录能力(`user_login`)

到本节前,自助注册的 app 是**纯 API key 身份**:`grants: []`、`redirect_uris: []`,fail-closed,根本不可能签出用户令牌。当每一张开放 API 面都是匿名读时那是对的默认;当某张面必须知道**是哪个用户**时它就是错的 —— 游戏时长是第一个。替代方案是继续在 OAuth 控制台手工建 client,那等于让人永远卡在每一个第三方集成的中间。

`POST /api/v1/dev/apps` 与 `PATCH /api/v1/dev/apps/:client_id` 接受可选的 `user_login`:

```json
{ "name": "Kurumi", "user_login": {
    "redirect_uris": ["http://127.0.0.1:53682/callback"],
    "scopes": ["openid", "profile"] } }
```

给出它 → app 置 `is_public=true`、`grants=["authorization_code","refresh_token"]`,scope 并入 `allowed_scopes`(`openid` 自动补)。**不给 → 完全保持原样**,本字段出现前注册的每个 app 行为逐字节不变。`user_login` 是**整体替换**而非 patch:否则删掉一个回调将永远做不到,而废弃的回调正是最该能删的东西。

**四道护栏**(开放自助注册的代价,一个都不能省):

1. **回调白名单**:只收 `https://`(且不是裸 IP)与 `http://` 到 `127.0.0.1` / `[::1]` 环回。拒绝通配、fragment(隐式流的令牌通道)、userinfo(`https://example.com@evil.com/cb` 在人眼里是前者)、以及到任何非环回主机的明文 http —— 授权码就走在这个 URL 里。**`localhost` 也拒**:它过主机名解析,可以被指向别处,`127.0.0.1` 不能。
2. **强制 PKCE**:桌面应用把二进制发给用户,里面没有秘密。标 `is_public` 即让 OAuth 服务在无 `code_challenge` 时**拒绝**它的授权码。环回回调按 **RFC 8252 §7.3 端口无关**匹配(端口是运行时才选的),scheme/host/path/query 仍精确匹配 —— 非环回 URI 永远走不到这个分支。
3. **保留名**:同意页把应用名显示在用户账号旁边,「NextMoe 官方助手」就是我们自己托管的钓鱼页。含 nextmoe / 未萌 / kungal / 官方 / official / admin 等片段一律拒。这是地板不是滤网(存心的冒充者会用同形字),配套的是同意页上不靠猜意图的**第三方标记**(`owner_user_id` 非空)。
4. **同意 scope 白名单**(`selfServiceUserScopes`):`openid` / `profile` / `email` / `playtime:read` / `playtime:write` / **`catalog:edit`(wave R3,2026-08-17 起)**。**`playtime:read` / `playtime:write` 仍可申请,但调 `/v1/playtime` 与 `/v2/me/playtimes` 不再需要它们**——任何已开通用户登录的应用都能读写该用户自己的时长。这两个词留在白名单里只是为了让旧授权 URL 不 400。**注意仍不在其中的**:`image:upload`、`artifact:upload` —— 自助注册不能向人索取花我们存储的权限。往这张表里加一项是**政策决定**,不是改配置;`catalog:edit` 就是这样一次政策决定,其代价已在面上收讫:第三方令牌永远 `ModerationCapped`(只能提案、不能裁决),且每用户未决提案帽 20(429)。写共享语料因此始终隔着一道人审。

它与 API key 的 scope 白名单(`selfServiceScopes`,2026-08-18 起只剩 `catalog:read`)**故意分开**:两者管的是不同凭证。一个说机器 key 匿名能干什么,另一个说应用**能向人要什么**。合并就等于让只读 key 的白名单去决定同意页的政策。

### 9.3 授权制 scope 的申请通道(2026-08-18)

`/dev/*` 自助面新增两条端点,把 `news:read` 的"联系平台"变成门户里的一次申请:

| 端点 | 说明 |
|---|---|
| `POST /api/v1/dev/scope-applications` | body `{scope, message}`,scope 必须 ∈ `grantableScopes`(现为 `news:read`) |
| `GET /api/v1/dev/scope-applications` | 当前用户的全部申请及状态 |

管理侧三条(`/api/v1/admin/devapi/scope-applications*`)与状态机、`(user, scope)` 唯一语义、三档铸 key 判据见 [02 §3.9](./02-public-api.md)。

**门户 UI**:铸密钥对话框(`components/keys/MintModal.vue`)的自助复选框下方多一行授权制条目——已批准即普通复选框,未申请 / 待审 / 被拒则复选框禁用并就地给出状态与「申请授权」按钮(`components/keys/ScopeApplyModal.vue`)。**审批不追加 scope 到已有 key**:批准后要重新铸一把带 `news:read` 的密钥,对话框那一行就是这件事发生的地方。

**管理台**:`apps/web` 的 `/devapi` 页新增「Scope 申请审核」面板(`components/devapi/ScopeApplications.vue`),列 pending(可切"看全部"),批准 / 拒绝(拒绝须填理由,理由原样回执给申请人)。

### 9.4 平台策略矩阵 + 应用审批流(2026-08-18)

到本节前,门户的自助面是**无级别的**:注册即建应用、建完即启用、启用即能铸 key。这在只有我们自己用的时候是对的默认;一旦第三方真的进来,平台就需要一个能在**不改代码、不改部署**的前提下收紧或临时关停某一步的旋钮。本节把这个旋钮做成一张矩阵。

**矩阵**(四能力 × 允许的 mode,判据表见 [02 §3.10](./02-public-api.md)):`app.create`(自助 / 需审批 / 关闭)、`app.manage`、`key.mint`、`scope.apply`(后三者只有自助 / 关闭)。默认全开 —— **平台出厂是开放的,策略只做收紧**,所以任何一行缺 override 都等于今天的行为,升级这一波对现有开发者是零可见变化。

**吊销永远不入闸**。关掉 `key.mint` 的场景是「先别再发新钥匙了」,不是「谁也别想止损」;把 revoke 一起关掉会让一次泄漏在策略打开之前无法收敛。

**为什么是 ren-only**:改矩阵不是日常运营动作(那是 `devapi.manage` 管的:调 tier、配额、审 scope 申请),而是**改平台对外承诺**——「现在还能不能自助注册」这句话对所有第三方同时生效。故新增 `devapi.policy_manage`,**只进 ren 捆**且标 `non_delegable`(先例:`oauth.permissions.manage`)。管理台的矩阵对 admin **可见但只读**,并明示「仅 ren 可改」——看得见才知道当下是什么政策,看不见只会让人反复去问。

**审批流**只加了三个状态、没有第四个:`approved` / `pending` / `declined`。`withdraw`(申请人撤回)**故意不做**——待审的申请撤回等价于停用一个从未启用的应用,而 `declined` → resubmit 已经覆盖了「想改了再来」这条真实路径;多一个状态就多一组迁移与四处 UI 分支,换不到任何新能力。

**门户表现**(`apps/developer`):dashboard 拉 `GET /dev/policies` → `approval` 时创建对话框顶部挂提示、提交后回执「已提交,等待平台审核」;`disabled` 时创建按钮禁用并说明原因。应用卡片与详情页对 `pending` / `declined` 挂状态 chip;`declined` 展示拒绝理由 + 「重新提交」按钮(→ resubmit 端点),两态都**隐藏「停用」**并在密钥区写「审核通过后可铸造密钥」。`key.mint=disabled` → 铸造 / 轮换禁用(吊销照常);`app.manage=disabled` → 编辑 / 停用禁用;`scope.apply=disabled` → 「申请授权」禁用。

**管理台表现**(`apps/web`):`/devapi` 页顶部为策略矩阵卡(`components/devapi/PolicyMatrix.vue`,改动走确认弹窗;选中「默认」那格即 `DELETE` 掉 override 行),应用列表加状态过滤(`enabled` / `pending` / `declined` / `disabled` / `all`,缺省 `enabled` 兼容现状)并在卡片上显示 review 状态,`components/devapi/PendingApps.vue` 是待审应用面板(通过 / 拒绝,拒绝须填理由)。新页 `/devapi/keys`(`components/devapi/Keys.vue`)是**跨全部应用的密钥清单**:按状态与应用过滤、分页,只展示前缀与后四位等元数据,行动作 rotate / revoke 直接复用既有 per-app 端点 —— 这一页刻意**不新增编辑端点**,「编辑 token」在这个平台上从来就只有轮换与吊销两个动作。

