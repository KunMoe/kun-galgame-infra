# MCP Server(公开只读面的 AI-agent 协议适配层)

> 拍板 2026-07-23(D4 提级 Phase 2,见 [01 §13](./01-design.md);派发-验收模式同 wave 08/09)。
> 一句话定位:把**已有的公开 /v1 只读面**同时暴露为 **MCP(Model Context
> Protocol)server**,让 AI 助手 / agent 用自然的工具调用直接查生态目录——
> **不新造任何数据面、权限面或计量面**。

## 1. 核心架构裁决:纯透传适配器(thin pass-through adapter)

MCP server 是公开 /v1 契约前面的一层**协议适配**,不是第二个 API:

- 每个 MCP tool 调用 = 一次对公开 /v1 面(`api.nextmoe.dev`)的 HTTP 请求,
  **原样转发调用方的 API key**;响应 JSON 即 tool result。
- 因此鉴权、tier、NSFW 可见性、限流、日配额、用量计量**全部天然复用**:
  流量落在同一个面、记在同一把 key 上(`/dev/usage` 里与直连流量无别)。
  MCP 层自身**零 authz 逻辑、零计量逻辑**——它连数据库都不碰。
- 面的版本化留在上游 `/v1`;tool 名不带版本。上游 expand→contract 纪律
  (02 §3.5)自动覆盖 MCP 消费者。

## 2. 形态与宿主

- 新 Go 服务 **`cmd/mcp`**(端口 **9285**),用 **官方 MCP Go SDK**
  (`github.com/modelcontextprotocol/go-sdk`;执行时核最新 tag,若官方 SDK
  能力缺口再评估 `mark3labs/mcp-go`,以官方优先)。
- Transport:**Streamable HTTP、stateless 模式**(无会话粘性,水平扩展与
  现有 Fiber 服务同治)。不做 stdio 分发(自托管用户 M2 再议)。
- 域名:**`mcp.nextmoe.dev`**(独立子域,Dokploy panel-domain 姿态,与
  developer-portal 同款单服务项目;`nextmoe.dev` 族命名约定见 README)。
- 上游 base 走 env(`KUN_MCP_UPSTREAM_BASE`,prod = api.nextmoe.dev 的
  内网服务地址),超时/重试保守(单次 30s,不重试非幂等——M1 全只读,
  幂等 GET 允许一次重试)。

## 3. 认证(M1)

- 调用方在 MCP endpoint 上带 `Authorization: Bearer nm_<api-key>`(各 MCP
  客户端的标准 header 配置即可)。MCP 层只做**形态检查**(缺失/非 `nm_`
  前缀 → 立即 MCP error,提示去 developer.nextmoe.dev 领 key);**真正的
  鉴权仍在上游面**(key 无效/超限时把面的 401/403/429 错误体转成带
  说明的 tool error 返回)。
- MCP 规范的 OAuth 2.1 授权流 = M2(第三方实际开放后,与 `dev:manage`
  同期评估);M1 的静态 key 模式对 agent 场景已充分。

## 4. 工具面(37 个 = catalog 面 34 + news 面 3;catalog 34 = M1 五个幸存 + `catalog_name_get` + canonical-W1 三件 + A2 八件 + wave-189 三件 + wave-196 两件 + wave-213 波 2 的作品子资源十二件,2026-08-21 与 canonical 轨 spec 同步)

> **wave 207 口径澄清**:本节所有**覆盖率分数**(下文的 34/37)的分母**始终是 catalog 面**,与工具总数不是一回事。平台的公开 spec 现共 **45 op = catalog 面 37 + playtime 面 5 + news 面 3**(catalog 面 25→37 是 wave 213 波 1 的十二条作品子资源);playtime 面**刻意不在 MCP 范围内**,理由见本节末尾那条。

> **2026-08-18**:news 面三条**全部进面**(下表末三行)。它们与 catalog 工具**不共用凭据前提**——`news:read` 不在 devapi 自助 scope 集内(`TestScopeNewsReadSelfServiceExcluded` 钉死),合作方只授权了索引;故三条工具的描述里各自写明授权制,不然模型只会稳定地拿到 403 而不知道为什么。**同日改动**:授予路径从"平台人工签发"改为**门户申请 + 平台审批**(见 [02 §3.9](./02-public-api.md)),`NewServer` 的 `instructions` 串同步改口(不再说 grants by hand,改说 apply in the developer portal),并加一句署名建议的英文等价表述——instructions 是模型在**任何工具调用之前**唯一读得到的说明,漏改会让它照着旧路径去建议用户联系平台。

> **wave 146(2026-07-30)**:`galgame_search` / `galgame_get` **随其上游 `/v1/galgame` 面一同退役**——该面现返回 `410 Gone`,继续注册这两个工具只会稳定地喂给调用方一个错误。后继:`catalog_search`(`type=works`)接自然语言搜索,`catalog_work_get` 接按 id 取详情。

| tool | 上游端点 | 说明 |
|---|---|---|
| `catalog_search` | `GET /v1/catalog/search` | 实体搜索,`type=names\|characters\|labels\|works`(works=跨媒介作品标题,r18 需 `nsfw=true`) |
| `catalog_work_get` | `GET /v1/catalog/works/{id}` | 注册行 + 可选 credits/relations(`include=credits,relations` 由该端点单次内联返回——MCP 层纯透传 `include`,不再并取子端点;+`nsfw`) |
| `catalog_lookup_external` | `GET /v1/catalog/lookup` | killer:`source=vndb&external_id=v19658` → work + 认领指针(+`nsfw`,默认 r18 命中 404) |
| `catalog_name_get` | `GET /v1/catalog/names/{id}` | 名义(credit-name 同人格分组;`include=credits` 附署名作品+角色) |
| `catalog_label_get` | `GET /v1/catalog/labels/{id}` | 厂牌/社团(intros[]/links[];`include=works`+`nsfw`) |
| `catalog_character_get` | `GET /v1/catalog/characters/{id}` | 角色(traits 按 `spoilers=0-2` 分级;`nsfw` 控 r18 作品+sexual 系 traits) |
| `catalog_works_list` | `GET /v1/catalog/works` | 批量浏览/过滤(content_rating/claimed/label/tag/series/platform/发售窗;`ids=` 批量水合;keyset 分页) |
| `catalog_changes` | `GET /v1/catalog/changes` | 增量同步变更流(keyset 游标存续轮询;entity_type=work) |
| `catalog_tag_get` | `GET /v1/catalog/tags/{id}` | 正典标签(跨源标签词表;`include=works` 附携带作品) |
| `catalog_works_search` | `GET /v1/catalog/works/search` | 作品产品检索(自由文本 + works-list 全过滤集;五档 sort、可选 facets、page 分页;`claim_state`/`content_limit`/`olang`/`search_intro`) |
| `catalog_calendar` | `GET /v1/catalog/calendar` | 发售月历单月(date ASC keyset;缺省=当前 Asia/Tokyo 月;`olang` 缺省 ja+zh* 族) |
| `catalog_calendar_pending` | `GET /v1/catalog/calendar/pending` | 月历「知年不知月」桶(缺省=当前 Asia/Tokyo 年) |
| `catalog_calendar_tba` | `GET /v1/catalog/calendar/tba` | 月历「已公布未定档」全局桶 |
| `catalog_labels_list` | `GET /v1/catalog/labels` | 厂牌词表浏览(`kind` 过滤;每行带 nsfw 感知 `work_count`;发现 label id 用) |
| `catalog_tags_list` | `GET /v1/catalog/tags` | 正典标签词表浏览(`tier`/`kind` 过滤;发现 tag id 喂给 works 过滤) |
| `catalog_engines_list` | `GET /v1/catalog/engines` | 引擎词表浏览(发现 engine id 喂给 `catalog_works_search`) |
| `catalog_engine_get` | `GET /v1/catalog/engines/{id}` | 引擎记录(名称 + nsfw 感知 `work_count` + 跨源 refs) |
| `catalog_series_list` | `GET /v1/catalog/series` | 系列词表浏览(`source=` 泳道过滤,开词表;发现 series id 用——系列**不进搜索索引**,此为唯一发现入口) |
| `catalog_series_get` | `GET /v1/catalog/series/{id}` | 系列记录(身份 + 源锚 + intros;`include_works` 附成员作品**按阅读顺序**,分页 `limit`/`offset`) |
| `catalog_stats` | `GET /v1/catalog/stats` | 全库计数(各媒介 LIVE 作品数 + 身份家族总量;无参数) |
| `catalog_label_relation_graph` | `GET /v1/catalog/labels/{id}/relation-graph` | 会社家族整图(nodes[]+edges[];服务端封顶 depth 4 / 60 节点,广度优先,无分页;`catalog_label_get.relations[]` 只有一跳) |
| `catalog_releases` | `GET /v1/catalog/releases` | 发售动态 release 粒度(date keyset;`date_from`/`date_to`/`platform`/`lang`/`olang`/`kind`/`official`/`content_limit`;`is_first` 分辨首发与再版) |
| `catalog_work_covers` | `GET /v1/catalog/works/{id}/covers` | 作品封面块单块分页(`limit`/`offset`/`nsfw`)。CDN 渲染不了的行**在分页前**丢弃,故短页 = 块取完;只要两个展示位走 `catalog_work_get.cover_slots` |
| `catalog_work_screenshots` | `GET /v1/catalog/works/{id}/screenshots` | 截图块(带尺寸 + thumbhash);每行自带 sexual/violence 等级,**只报告不过滤** |
| `catalog_work_tags` | `GET /v1/catalog/works/{id}/tags` | 源标签块(count DESC → name → source);映射行带 canonical_id/tier/kind + nsfw 感知 `work_count`。**十二条里唯一带 `spoilers` 的**(0-2,超限行整行剔除、不占页位) |
| `catalog_work_characters` | `GET /v1/catalog/works/{id}/characters` | 角色花名册块(main → secondary → appears → 仅署名项,带配音署名)。**无 `spoilers` 参数**——每行自带 spoiler 等级,由调用方分级 |
| `catalog_work_credits` | `GET /v1/catalog/works/{id}/credits` | 职员署名块按 role 分组,但**分页按署名行不按组**:跨页的 role 在两页各出现一次、各带该页切片,拼页须按 `role_key` 合并 |
| `catalog_work_releases` | `GET /v1/catalog/works/{id}/releases` | 该作品的发售行块(release id 升序,各带源锚与自己的 `labels[]`);跨作品时间线走 `catalog_releases` |
| `catalog_work_intros` | `GET /v1/catalog/works/{id}/intros` | 简介块(一语言一行,选举同母面:源写胜过机翻,机翻行**打标**不隐藏) |
| `catalog_work_ratings` | `GET /v1/catalog/works/{id}/ratings` | 分源评分块(**work-detail 投影**,带完整直方图与离散度——works-list 的 ratings 块会丢掉);分数留各源原生标尺 |
| `catalog_work_relations` | `GET /v1/catalog/works/{id}/relations` | 关联作品块(母面只在 `include=relations` 时给);无 `nsfw` 时 r18 关联端**整条丢弃**而非置空,`next_offset` 数幸存行 |
| `catalog_work_series` | `GET /v1/catalog/works/{id}/series` | 所属系列块;`member_count` 是系列**全部**成员数而非本页 |
| `catalog_work_links` | `GET /v1/catalog/works/{id}/links` | 非身份外链块(官网/Steam/X 等)。**地址不是锚**,身份锚在 `refs[]`;dlsite/dmm 无法由裸 code 还原商店 URL,按设计不在此面 |
| `catalog_work_engines` | `GET /v1/catalog/works/{id}/engines` | 引擎块,每行带 nsfw 感知 `work_count`(= 该调用方用 `catalog_works_search engine_id=` 真能翻到的作品数) |
| `news_list` | `GET /v1/news` | 合作方资讯索引(keyset;`source`/`lane`/`work_id`/`published_after`/`published_before`)。**索引非镜像**:只有 preview + banner + `source_url`,正文永不出面。**需 `news:read`(授权制)** |
| `news_sources` | `GET /v1/news/sources` | 来源注册表(key / 名称 / 主页 / 专栏入口 / publisher uid / 归属文案),无参数。**需 `news:read`(授权制)** |
| `news_get` | `GET /v1/news/{id}` | 单条资讯;撤回或上游消失后 404 是契约而非查不到。**需 `news:read`(授权制)** |

- **catalog 覆盖面(34/37:仅剩三条「有意留白」;分母是 catalog 面,平台另
  5 op 属 playtime 面,见下条)**:公开 catalog 面
  现共 37 op,上表覆盖 34——**没覆盖的恰好就是本条末尾那三条有意留白**,
  再无「待裁定」项。上一波记为「待裁定」的**作品子资源十二条**
  (`works/{id}/covers` 等,见 [02 §3.2](./02-public-api.md))已由 owner 裁定
  **全部收进工具面**(wave 213 波 2,2026-08-21)。裁定理由:它们分页的确实是
  `catalog_work_get` 已经整块返回的东西,但那正是收面的价值——一个数据丰富的作品
  带 `include=relations,credits` 是 50 KB,身份核心不到十分之一,而模型多数时候
  只要其中一块,十二条让它按块取而不必为一个封面吞下整个作品;十二条全是 GET,
  参数与母面逐字一致(`limit`/`offset`/`nsfw`),合乎纯透传红线。**逐参对齐时踩到的一处**:
  `characters` **没有** `spoilers` 参数(花名册每行自带 spoiler 等级由调用方分级),
  十二条里只有 `tags` 带 `spoilers`——按「母面有的子面都有」想当然会造出一个上游
  根本不认的参数。上一波记为「待裁定」的 `stats` 与 `series` 已由 owner
  裁定收进工具面(wave 189,2026-08-07),且 `series` 实收**两条**而非一条——
  系列不进任何搜索索引,只有 `series/{id}` 详情道的话调用方**永远拿不到 id**,
  浏览道是它的唯一发现入口,两条必须成对进面。上一波记为「待裁定」的
  `GET /v1/catalog/labels/{id}/relation-graph` 与 `GET /v1/catalog/releases` 已由
  owner 裁定收进工具面(wave 196,2026-08-08)。两条的收面理由都是**实测**而非直觉:
  relation-graph 服务端封顶 depth 4 / 60 节点且不分页,全库最大连通分量只触及 32 条边,
  载荷有硬上界,而它答的「某社旗下有哪些牌子 / 这个牌子归谁」是多跳问题——单跳的
  `catalog_label_get.relations[]` 只能让模型自己猜下一步走谁。`releases` 起初被判为
  「大基数列表面、收益低」,该判断经实测**推翻**:它有 13 个过滤器(日期区间 / platform /
  lang / olang / kind / official / content_limit / cursor / limit / include),载荷由
  `limit` 而非语料决定(31.9 万 release,一个「2024 年 Windows 日语」这样的现实问法
  收敛到 1,843 条,默认页量 20),且它答的平台 / 语言 / 版本类型 / 官方性是 calendar
  三桶按构造答不了的——calendar 把作品放在**最早**发售月且只显示一次。上一波记为「待裁定」的 A2 八条(calendar 三桶 / taxonomy 列表三条 /
  `engines/{id}` / `works/search`)已由 owner 裁定收进工具面(wave 7,2026-07-30),
  八条全是 GET,合乎纯透传红线。**有意留白**仍是三条:`POST /v1/catalog/lookup/batch`
  (批量外部 id 水合)、`GET /v1/catalog/redirects`(合并事件 keyset 流,供镜像清理
  存量 id)、`POST /v1/catalog/resolve`(旧 id→正典 id 批量扁平化)——它们服务的是
  **镜像维护 / 批量同步**型消费者,应直连 HTTP 面:单轮 LLM tool call 没有批量、
  也没有存量 id 维护语义;小批量水合已由 `catalog_works_list` 的 `ids=` 覆盖,单个
  外部 id 由 `catalog_lookup_external` 覆盖;且 `lookup/batch` 与 `resolve` 是
  POST,而 mcpface 传输是 GET 纯透传。

- **playtime 面:已裁定不进 MCP(wave 207,非「还没做」)**。平台的第二个公开面
  `/v1/playtime`(5 op,见 [02 §3.8](./02-public-api.md))**整族出界**,理由是它与
  §1 的红线正面冲突:MCP 层是**纯透传**适配器,把调用方那把 `X-API-Key` 原样转发给
  上游、自己零 authz 零计量**连数据库都不碰**;而 playtime 是**用户 Bearer 令牌**认证的
  **写**面,写的是某个具体人的个人记录。把一枚用户令牌穿过 LLM 的 tool 循环去写他的
  个人数据,不在 MCP 的授权范围内——这需要的是 MCP 规范的 OAuth 2.1 流(§3 里的
  M2),而不是把 `Authorization` 头换一种内容继续透传。触发条件与 M2 同期:真出现
  「让 agent 帮我同步游玩库」的外部需求时,连同写面工具一起评估,而不是先把面开了。
  在那之前,**覆盖率分数不把这 5 条算进分母**。

- **r18 姿态 = 调用方自控**(104 波所定;wave 213 波 2 曾改为能力位门控,该能力位已于
  2026-08-25 退役,姿态回到 104 波原样):catalog 系工具是 `nsfw=true` 显式开、默认
  全部隐藏——LLM 消费者不显式要就永远看不到 r18,而**要了就一定拿得到**,不再有
  凭证维度的 403。每个工具的 `nsfw` 参数描述与 `instructions` 串里的
  「requires an API key with the NSFW capability」已一并删除:留着的话模型会以为
  自己缺权限,把一次空结果读成「被拒绝」而反复换问法重试。(旧 galgame 系工具的
  `content_limit`+`galgame:nsfw` scope 姿态已随 /v1/galgame 摘牌一并退役;
  `catalog_works_search` 的 `content_limit` 是编辑展示轴,与 r18 轴无关。)

- tool description 用英文、面向 LLM 写清「何时用哪个」(lookup vs search
  的分工是重点:有外部 id 用 lookup,自然语言用 search)。
- 输入 schema 逐参对齐上游 query 参数(分页参数透传,默认页量保守)。
- **不做**的(明确出界):redirects/resolve/lookup/batch(镜像维护面,理由见
  上面的覆盖说明)、**playtime 面整族**(wave 207,理由见上)、resources/prompts(M2)、
  任何写面(Phase 3 submit 开放后随 OAuth 一起评估)。`changes`(canonical-W1)与 calendar 三桶(wave 7,收的是
  **catalog 面**的月历;当年出界的是已退役的 galgame 面月历)原属此列,现均已
  进面(见上表)。

## 5. 运维与部署

- 独立 Dokploy 单服务项目(照抄 developer-portal 的 panel-domain +手动
  Deploy 姿态,`docker-compose.mcp.yml`);镜像走现有 CI 矩阵。
- healthz 照平台惯例;结构化日志记 tool 名 + 上游状态码 + 时延,
  **永不记 key 明文**(fingerprint 前 8 hex)。
- 冒烟:MCP `initialize` + `tools/list` + 一次 `catalog_search` 真调用。(2026-08-21 起 `tools/list` 应回 **37** 工具 = catalog 34 + news 3;playtime 面按 §4 裁定不进面,故这个数不随它变。冒烟调用仍走 `catalog_search`——早先写的 `galgame_search` 已随 `/v1/galgame` 面于 wave 146 退役;**不要拿 news 工具冒烟**,冒烟用的 key 没有 `news:read`,一个正确的 403 会被读成部署失败。)

## 6. 阶段

- **M1(本波)**:§1-§5 全部;门户 docs 页加「AI/MCP 接入」一节(端点、
  key 配置示例:Claude Code / Claude Desktop / 通用 MCP 客户端片段)。
- **M2(触发式)**:MCP resources(work 页面作为资源)、prompts、OAuth
  2.1、stdio 自托管包、写面工具。触发条件 = 真实外部消费者出现。
