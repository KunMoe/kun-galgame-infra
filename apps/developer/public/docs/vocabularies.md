# Vocabularies

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## 词表（共 22 个）

封闭词表（`closed`）不加不减成员就是它的承诺——你的 `switch` 可以没有 `default`。开放词表相反：提前告诉你会有新值，客户端必须容忍没见过的取值。运行时以 `https://api.nextmoe.dev/v2/vocabularies` 为准，它比任何文档都新，且不需要凭据。

### `medium`

Closed · 7 tokens

| value | display_name | description |
| --- | --- | --- |
| `galgame` | Galgame | A galgame / visual novel work. |
| `manga` | Manga | A manga work. |
| `novel` | Novel | A light novel or other prose work. |
| `anime` | Anime | An anime work. |
| `asmr` | ASMR | An ASMR work. |
| `doujin_game` | Doujin game | A doujin game that is not classified as galgame. |
| `music` | Music | A music work. |

### `content_rating`

Closed · 3 tokens

| value | display_name | description |
| --- | --- | --- |
| `all_ages` | All ages | Rated suitable for all ages. |
| `sensitive` | Sensitive | Sexual or violent content below the r18 line. |
| `r18` | R18 | Adult-only content. |

### `sexual`

Closed · 3 tokens

| value | display_name | description |
| --- | --- | --- |
| `safe` | Safe | No sexual depiction. |
| `suggestive` | Suggestive | Suggestive sexual depiction. |
| `explicit` | Explicit | Explicit sexual depiction. |

### `violence`

Closed · 3 tokens

| value | display_name | description |
| --- | --- | --- |
| `tame` | Tame | No violent depiction. |
| `violent` | Violent | Violent depiction short of brutality. |
| `brutal` | Brutal | Brutal violent depiction. |

### `spoiler`

Closed · 3 tokens

| value | display_name | description |
| --- | --- | --- |
| `none` | None | Not a spoiler. |
| `minor` | Minor | Minor spoiler. |
| `major` | Major | Major spoiler. |

### `gender`

Closed · 3 tokens

| value | display_name | description |
| --- | --- | --- |
| `male` | Male | Male. |
| `female` | Female | Female. |
| `other` | Other | Recorded as other. Absence of a record is null, not a value. |

### `blood_type`

Closed · 4 tokens

| value | display_name | description |
| --- | --- | --- |
| `a` | A | Blood type A. |
| `b` | B | Blood type B. |
| `ab` | AB | Blood type AB. |
| `o` | O | Blood type O. |

### `release_status`

Closed · 5 tokens

| value | display_name | description |
| --- | --- | --- |
| `released` | Released | The release has shipped. |
| `dated` | Dated | A date is known and the release has not shipped. |
| `announced` | Announced | Announced without a usable date. |
| `cancelled` | Cancelled | Cancelled. |
| `unknown` | Unknown | We have not determined the status. |

### `claim_state`

Closed · 5 tokens

| value | display_name | description |
| --- | --- | --- |
| `live` | Live | The claim is live. |
| `draft` | Draft | The claim is a draft. |
| `pending` | Pending | The claim is waiting for review. |
| `declined` | Declined | The claim was declined. |
| `hidden` | Hidden | The claim is hidden. |

### `content_limit`

Closed · 2 tokens

| value | display_name | description |
| --- | --- | --- |
| `sfw` | SFW | Safe to render in an SFW context. |
| `nsfw` | NSFW | Not safe to render in an SFW context. |

### `tier`

Closed · 3 tokens

| value | display_name | description |
| --- | --- | --- |
| `core` | Core | Core vocabulary tag. |
| `longtail` | Longtail | Long-tail tag. |
| `hidden` | Hidden | Hidden from default listings. |

### `title_kind`

Closed · 3 tokens

| value | display_name | description |
| --- | --- | --- |
| `official` | Official | Official title. |
| `alias` | Alias | Alternate title. |
| `abbreviation` | Abbreviation | Abbreviated title. search_hint is internal and is not in this vocabulary. |

### `alias_kind`

Closed · 2 tokens

| value | display_name | description |
| --- | --- | --- |
| `translation` | Translation | A translated name. |
| `spelling_variant` | Spelling variant | A spelling variant. search_hint is internal and is not in this vocabulary. |

### `roster_role`

Closed · 4 tokens

| value | display_name | description |
| --- | --- | --- |
| `unknown` | Unknown | The roster role was not recorded. |
| `main` | Main | A main character. |
| `secondary` | Secondary | A secondary character. |
| `appears` | Appears | A character who appears. |

### `attribution_role`

Closed · 4 tokens

| value | display_name | description |
| --- | --- | --- |
| `circle` | Circle | Circle / doujin group attribution. |
| `publisher` | Publisher | Publisher attribution. |
| `developer` | Developer | Developer attribution. |
| `brand` | Brand | Brand attribution. |

### `company_kind`

Closed · 6 tokens

| value | display_name | description |
| --- | --- | --- |
| `game_brand` | Game brand | A game brand. |
| `bunko` | Bunko | A bunko / imprint. |
| `publisher` | Publisher | A publisher. |
| `anime_studio` | Anime studio | An anime studio. |
| `doujin_circle` | Doujin circle | A doujin circle. |
| `group` | Group | A group. |

### `tag_kind`

Closed · 2 tokens

| value | display_name | description |
| --- | --- | --- |
| `content` | Content | A content tag. |
| `meta` | Meta | A meta tag. |

### `release_kind`

Closed · 5 tokens

| value | display_name | description |
| --- | --- | --- |
| `default` | Default | The default / unspecified edition. |
| `digital` | Digital | A digital edition. |
| `physical` | Physical | A physical edition. |
| `trial` | Trial | A trial / demo edition. |
| `patch` | Patch | A patch, including fan translations. |

### `member_kind`

Closed · 5 tokens

| value | display_name | description |
| --- | --- | --- |
| `unknown` | Unknown | The series-member role was not recorded. |
| `main` | Main | The main entry of the series. |
| `fandisc` | Fandisc | A fandisc. |
| `side_story` | Side story | A side story. |
| `collection` | Collection | A collection / omnibus. |

### `problem_domain`

Closed · 6 tokens

| value | display_name | description |
| --- | --- | --- |
| `platform` | Platform | Errors any face may emit. |
| `catalog` | Catalog | Catalog-face errors. |
| `me` | Me | User-facing /v2/me errors. |
| `moderation` | Moderation | Moderation-face errors. |
| `news` | News | News-face errors. Both codes come from the source-row-as-grant model on /v2/me/news. |
| `store` | Store | Store-face errors from the purchase-link minter. |

### `sources`

Open · 20 tokens

| value | display_name | description |
| --- | --- | --- |
| `user` | User | Manual curation, not an import source. |
| `vndb` | VNDB | The VNDB catalog. |
| `bangumi` | Bangumi | The Bangumi catalog. |
| `dlsite` | DLsite | DLsite storefront identities. |
| `erogamescape` | ErogameScape | ErogameScape. |
| `anilist` | AniList | AniList. |
| `mal` | MyAnimeList | MyAnimeList. |
| `steam` | Steam | Steam storefront. |
| `official_site` | Official site | A publisher or brand official site. |
| `twitter` | Twitter / X | A Twitter / X identity. |
| `pixiv` | Pixiv | Pixiv. |
| `curated` | Curated | First-party curated / human lane. |
| `upscale` | Upscale | First-party AI-upscaled cover derivation. |
| `cien` | Ci-en | Ci-en creator-support platform. |
| `dmm` | DMM | DMM storefront. |
| `web` | Web | A generic web page. external_id is the full URL. |
| `getchu` | Getchu | Getchu.com retailer pages. |
| `derived` | Derived | First-party machine inference over catalog facts. |
| `nextmoe` | NextMoe | First-party measurements aggregated from our users. |
| `howlongtobeat` | HowLongToBeat | HowLongToBeat playtime and rating aggregates. |

### `relation_types`

Open · 16 tokens

| value | display_name | description |
| --- | --- | --- |
| `adaptation_of` | Adaptation of | This work is an adaptation of the other. |
| `sequel_of` | Sequel of | This work is a sequel of the other. |
| `side_story_of` | Side story of | This work is a side story of the other. |
| `fandisc_of` | Fandisc of | This work is a fandisc of the other. |
| `collects` | Collects | This work collects the other. |
| `remake_of` | Remake of | This work is a remake of the other. |
| `same_series` | Same series | The two works belong to the same series. Symmetric. |
| `same_setting` | Same setting | The two works share a setting. Symmetric. |
| `crossover_with` | Crossover with | The two works crossover. Symmetric. |
| `shares_character` | Shares character | The two works share a character. Symmetric. |
| `alternative_setting` | Alternative setting | The two works share characters in a different setting. Symmetric. |
| `alternative_version` | Alternative version | The two works are alternative versions. Symmetric. |
| `imprint_of` | Imprint of | This company is an imprint of the other. |
| `renamed_from` | Renamed from | This company was renamed from the other. |
| `subsidiary_of` | Subsidiary of | This company is a subsidiary of the other. |
| `member_of` | Member of | This company is a member of the other. |

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/vocabularies
