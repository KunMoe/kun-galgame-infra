# AI / MCP 接入

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

NextMoe 开放 API 同时以 MCP（Model Context Protocol）server 暴露：端点 https://mcp.nextmoe.dev/mcp，Streamable HTTP、stateless，带上同一把 API 密钥即可。它是一层纯透传适配——每次工具调用就是一次对公开 /v2 端点的请求，鉴权、限流、配额与用量与直连毫无区别；资讯、词表、错误码注册表与目录规模统计在这里同样不要凭据。

## 工具（37 个）

- `catalog_search`：按名字搜身份图谱实体：names（人物名义）/ characters / labels / works（跨媒介作品标题，r18 需 nsfw=true）。
- `catalog_work_get`：按 catalog work id 取注册行，include=credits,relations 并取子块。
- `catalog_lookup_external`：外部 id 反查（如 source=vndb, external_id=v19658）——手握外部 id 时首选。
- `catalog_name_get`：按 id 取名义（credit-name 同人格分组），include=credits 附署名作品与角色。
- `catalog_label_get`：按 id 取厂牌 / 社团（include=works 附归属作品）。
- `catalog_character_get`：按 id 取角色（traits 按 spoilers=0-2 分级；nsfw 控 r18 作品与 sexual 系 traits）。
- `catalog_works_list`：批量浏览 / 过滤作品注册表（评级 / 厂牌 / 标签 / 系列 / 平台 / 发售窗，keyset 分页，ids= 批量水合）。
- `catalog_changes`：增量同步变更流——存下 next_cursor，下次轮询只拿变化的部分。
- `catalog_tag_get`：按 id 取正典标签（跨源标签词表），include=works 附携带作品。
- `catalog_works_search`：作品产品检索：自由文本 + works-list 全过滤集，五档排序、可选 facets 分面计数、page 分页（组合「查询 + 过滤」时优先用它，纯名字检索用 catalog_search）。
- `catalog_calendar`：发售月历单月（缺省为当前 Asia/Tokyo 月；olang 缺省收敛到 ja + zh* 族，olang=all 放开）。
- `catalog_calendar_pending`：月历「知年不知月」桶（缺省为当前 Asia/Tokyo 年）。
- `catalog_calendar_tba`：月历「已公布未定档」全局桶。
- `catalog_labels_list`：浏览厂牌 / 社团词表本身（kind 过滤，每行带 nsfw 感知 work_count）——用来发现 label id。
- `catalog_tags_list`：浏览正典标签词表本身（tier / kind 过滤）——用来发现 tag id 再喂给作品过滤。
- `catalog_engines_list`：浏览引擎词表本身——用来发现 engine id 再喂给 catalog_works_search。
- `catalog_engine_get`：按 id 取引擎记录（名称 + nsfw 感知 work_count + 跨源 refs）。
- `catalog_series_list`：浏览系列词表本身（source= 泳道过滤：curated / derived / dlsite，每行带 nsfw 感知 work_count）——系列不进搜索索引，这是发现 series id 的唯一入口。
- `catalog_series_get`：按 id 取系列（身份 + 源锚 + 简介），include_works 附成员作品并按阅读顺序排列——回答「这个系列按什么顺序玩」。
- `catalog_stats`：全库计数：各媒介 LIVE 作品数 + 身份家族总量（无参数）。
- `catalog_label_relation_graph`：一次拿到一个厂牌周围的整个会社家族（母公司 / 子品牌 / 文库 / 继承），nodes[] + edges[]。catalog_label_get 的 relations[] 只有一跳，问「某社旗下有哪些牌子」用这个。服务端封顶 depth 4 / 60 节点，不分页。
- `catalog_releases`：发售动态的 release 粒度：每一条发售行各自成项，移植版 / 复刻 / 中文化都看得见（calendar 只把作品放在最早发售月且只显示一次）。可按日期区间、平台、发行语言、版本类型、官方性过滤；is_first 分辨首发与再版。
- `catalog_work_covers`：作品的封面块（单块分页，只要这一块时优先于 catalog_work_get）。CDN 渲染不了的行在分页前就被丢掉，所以短页 = 块取完了，不是被过滤了。只要两个展示位用 catalog_work_get 的 cover_slots。
- `catalog_work_screenshots`：作品的截图块（单块分页，带尺寸与 thumbhash）。每行各自带 sexual / violence 等级——此面只报告不过滤，渲染门由调用方自己把。
- `catalog_work_tags`：作品的源标签块（单块分页，count DESC → name → source）。命中正典映射的行带 canonical_id / tier / kind 与 nsfw 感知 work_count，未映射的行不带；`spoilers=0-2` 设标签剧透上限（超限行整行剔除、不占页位）。
- `catalog_work_characters`：作品的角色花名册块（单块分页，main → secondary → appears → 仅署名项，带配音署名）。此面**不设**剧透上限、也**没有** spoilers 参数——每行自带 spoiler 等级，由调用方分级。
- `catalog_work_credits`：作品的职员署名块，按 role 分组。**分页按署名行不按组**：跨页的 role 会在两页各出现一次、各带该页的切片，拼页时要按 role_key 合并组。
- `catalog_work_releases`：作品的发售行块（单块分页，release id 升序，各带源锚与自己的 labels[]——移植版 / 英文版各自的发行商在这里）。跨作品的发售时间线用 catalog_releases。
- `catalog_work_intros`：作品的简介块（单块分页，一语言一行）。选举同母面：源写的胜过机翻的，机翻行是**打标**不是隐藏——引用成「官方说法」前先看标。
- `catalog_work_ratings`：作品的分源评分块（单块分页，带完整投票直方图与离散度——works-list 的 ratings 块会把这些丢掉）。分数留在各源原生标尺，永不混算成一个数。
- `catalog_work_relations`：作品的关联作品块（单块分页）——catalog_work_get 只在 `include=relations` 时才给。不带 nsfw 时 r18 关联端是**整条丢弃**而非置空，next_offset 数的是幸存行。续作 / FD 也能经 catalog_work_get 的 series_siblings（传递闭包）拿到。
- `catalog_work_series`：作品所属系列块（单块分页）。member_count 是系列的**全部**成员数而非本页——顺着它用 catalog_works_list series_id= 或 catalog_series_get include_works（后者才给阅读顺序）。
- `catalog_work_links`：作品的非身份外链块（官网 / Steam / X 等，单块分页）。这些是**地址不是锚**，身份锚在 catalog_work_get 的 refs[]；dlsite / dmm 这类无法由裸 code 还原商店 URL 的源按设计不在此面，仍从 refs[] 走。
- `catalog_work_engines`：作品的引擎块（单块分页，每行带 nsfw 感知 work_count = 该调用方用 catalog_works_search engine_id= 真能翻到的作品数）。
- `news_list`：合作媒体的 Galgame 资讯索引（按来源 / 泳道 / 关联作品 / 发布时间窗过滤，keyset 分页）。只有标题、摘要与题图，正文永不下发——每条恒带来源与 source_url，读全文要回到媒体自己的站点。
- `news_sources`：资讯来源注册表：每家媒体的 key、名称、主页、专栏入口，以及该渲染的归属文案。无参数。
- `news_get`：按 id 取单条资讯。已撤回的、上游原文已消失的条目返回 404——这是契约不是查不到，别重试，也别拿缓存副本顶上。

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/mcp
