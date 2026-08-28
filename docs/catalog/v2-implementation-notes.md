# v2 implementation notes

This page records **binds that differ from `refs/api-v2`**. It is not a substitute for the design spec. Code + `docs/catalog/v2-openapi.yaml` are the machine-readable contract.

Date: 2026-08-24, GA declared 2026-08-25 (stage 9); news write face added 2026-08-25. v1 is unchanged; the v1 sunset is still not scheduled here.

## Done on this branch

- Protocol, problems, vocabularies, collection contract, repr types, CI gates G1–G16.
- Catalog read: works (list/detail/12 subs), companies+graph, tags, series, engines, releases, characters, credit-names, persons, traits, search, calendar, changes, redirects, stats, schemas/{object}, news.
- `GET /v2/catalog/works` binds 05 §6.1 filters (`q=`, `company_id=`, `tag_id=`, `series_id=`, `engine_id=`, `olang=`, dates, claim/content axes). `q=` / search sorts / `facets=` use Meili (`WorksSearch`); other filter combinations use the live registry (`WorksList`). `sort=relevance` requires `q=`.
- `GET /v2/catalog/characters/{id}/appearances` is the character reverse-lookup collection (roster_role, spoiler, voices). Company reverse lookup is `works?company_id=`. Staff reverse lookup stays `credit-names/{id}/credits`.
- Character `view=full` carries D30 attributes (`gender`, `birthday` as `MM-DD`, measurements, `blood_type` as `a|b|ab|o`, `instance_of_id`). `description` / `extra` / `field_provenance` stay out, as D30.
- `/v2/me/playtimes` GET/PUT/DELETE and POST 207 batch; `/v2/me/cover-votes` GET/PUT/DELETE.
- `/v2/me/claims` list (keyset on last claim-event id)/create/get/withdraw; `/v2/me/proposals` list/create/get/patch/amend.
- `/v2/moderation/claims` queue + GET by id + decisions; `/v2/moderation/proposals` queue + decisions; reverts; snapshots. Queue and GET are site-fenced (`SITE_NOT_BOUND` when the token client has no catalog site).
- User token on `/v2/me` and `/v2/moderation`, not an application key; `private, no-store`. JWT `roles` are copied into the handler context so `HasPerm` matches v1.
- **D35** (2026-08-25): claim state machine completed (`PATCH {state: live|pending}`, decisions `ban`/`unban`, `catalog.claim.review` enforced); public edit history at `/v2/catalog/revisions` + `/v2/catalog/proposals`; `POST /v2/me/edit-images`; `GET /v2/moderation/proposals/{id}`. The v1 `claim-events/feed` and S2S `works/claim` stay on v1 by decision (D35d) and must be named individually in any `/api/v1/catalog/**` retirement.
- `/v2/me/news` list/create/get/patch (D34, 2026-08-25). Source-row-as-grant: `news_source.publisher_uid` is the authorisation. No schema change — `news/service/submission.go` writes the existing `kun_news` tables. Public read DTO (`repr.NewsItem`) is untouched.
- **Read-parity wave (2026-08-26, from the forum's v1 fallback report)**: rate limiting is now keyed to the application key's tier (deviation 29); the entity faces grow the data the v1 faces already carried (deviation 30); `works?refs=` that resolves nothing now answers an empty batch page + `missing[]` instead of silently dropping the IDs filter and returning page 1 of the whole catalogue (the same guard every other entity list already had).
- **Read-parity wave 3 (2026-08-27, four verified gaps from the forum)**: the spoiler ceiling is reachable at last — `spoiler=none|minor|major` on `works/{id}`, `works/{id}/tags` and `characters/{id}`, where v2 had hardcoded 0 (deviation 34); the company graph's `include=logo` stops being validated-then-discarded and puts the brand mark on the nodes that have one (deviation 35); `characters/{id}` stops throwing away the aliases, intros and refs `GetCharacter` already fetched, via `include=aliases,intros,refs`; and `lang` — the language of `display_name`, unreconstructable from `localized{}` — joins the company, credit-name and character shapes on every lane (deviation 36). Additive on the v2 wire; the only v1 change is one new optional key on the labels list projection.
- **Read-parity wave 2 (2026-08-26, the four gaps the forum still had after the first wave)**: the ratings block carries its vote histogram and spread on the detail faces (deviation 31); the work `companies` block carries the brand logo, tag detail takes `include=intros`, series grows `has_nsfw` plus `include=intros,refs` (deviation 32); `credit-names/{id}` becomes the staff read surface with person-level scalars and six include blocks, and its credits sub-face names the voiced character (deviation 33). Additive only — no v1 DTO, service or schema change.

## Spec deviations (with reason)

1. **`series` and `traits` `refs=`** — `catalog_external_ref` has no entity_type for series or trait (`constants.go` entity types stop at engine=8). Unknown refs go to `missing[]`. Not a new table.

2. **Character full fields use `omitempty`** — D30 says unrecorded is JSON `null`. `view=basic` must not emit those keys (default-thin). Unrecorded values on `view=full` are therefore omitted rather than `null`. Same projection trick as work `include=` blocks.

3. **Trait JSON uses `is_sexual`, not `sexual`** — G8: `Screenshot.sexual` is `*string` (`safe|suggestive|explicit`). A bool cannot share the property name. Work tags already use `is_sexual`.

4. **Cover vote is `up` only** — v1 stores one ballot per (user, work). There is no down column. `vote=down` is 422.

5. **Playtime GET empty is 404** — v1 returned 200 + null. v2 has no envelope; a missing row is `NOT_FOUND`.

6. **Playtime delete** — v1 had no DELETE. v2 DELETE removes `catalog_user_playtime` rows for that (user, work). Additive service method, existing table.

7. **`blood_type` public tokens are lowercase** (`a|b|ab|o`) per 04-representation. D30's table wrote `A|B|AB|O` as domain letters; the editing engine stores int16. Public JSON follows 04.

8. **Claim POST mint needs `display_name`** — D33. `SubmitWork` cannot mint without `catalog.work.display_name`. `refs` that miss `catalog_external_ref` become `catalog.work.links` URLs via the existing source URL templates. `work_id` and `refs` both absent is 422 even if `display_name` is present.

9. **Moderation queues and `/v2/me/proposals` are keyset-paginated** on event/proposal id (`cur_` + over-fetch).

10. **Works list `include=` hydrates the list-capable subset** (`titles`, `intros`, `companies`, `ratings`, `covers` slots, `refs`). Other FULL_SET tokens stay on detail / sub-resources. Facets on the SQL browse path return empty buckets; named facet counts come from Meili. **Amended 2026-08-27:** this entry used to read "SQL `include_total` is omitted (search path returns Meili `total`)". The SQL lane now honors `include_total` with an exact `count(*)` over the same predicate set the page query uses, so `/v2/catalog/works` publishes `total` on both lanes. The old behavior was never defensible as a deviation: `include_total` was parsed and validated on the SQL lane (a bogus value 400s) and the published envelope doc on `List.total` says "Present only when `include_total=true`. Same visibility gate as items." — which contradicted a lane that silently never published it, while every other registry list did. It also hard-blocked the forum's flip, whose pagination is `Math.ceil(total/limit)` and read `null` as zero pages. The COUNT is **flag-gated**, unlike the taxonomy lanes' unconditional count, because the works predicate set is `EXISTS`-subquery heavy (label rollup, tag maps, series, engine, platform, release windows); it follows the `ReleaseFeedMeta` / `CalendarMeta` precedent instead. It is computed from the filter predicates **before** the cursor predicate is appended, so paging does not shrink it. The all-miss `ids=`/`refs=` batch now publishes `total: 0` like every other entity lane rather than dropping the key.

11. **`release_status=` is not bound** on `GET /v2/catalog/works`. It is in 05 §6.1; v1 list did not have it either. Calendar remains the status filter.

12. **Character reverse lookup is `/characters/{id}/appearances`**, not `works?character_id=`. Appearance rows carry edge metadata the works collection cannot. This fills the 03 hole left when S2S `characters/{id}/works` was 410'd.

13. **`PATCH /v2/me/claims/{id}` pre-reads the claim site-fenced, not owner-fenced** (D35a). The owner check belongs to `ClaimLifecycleService.Act` via `RequireOwner`, which adopts an unowned row for its first claimant. Reading the current state through the owner-scoped `ClaimsByActor` query instead made adopt-and-publish — the most frequent claim write in production — answer 404 for every machine-imported draft.

14. **`status` is added to the G8 named exception list** (`gates_repr.go`, D34a). 06 §7.2 / 03 §2.4 spell the news lifecycle member `status`, while `Problem.status` and `playtime_batch_item.status` are HTTP status codes and stay integers. This is the same concept 07 §2.1 already admits for `state`; the list grows by one, with the reason recorded beside it.

15. **Claim and proposal ETags are domain validators emitted as real `ETag` headers** (D35a). `"c<work_id>.<state>"` and `"p<id>.<updated_unix>"`. Before this, claim decisions validated `If-Match` against `"c<work_id>"` while the GET returned a body digest, so no client could obtain a value that passed; the test hardcoded the unobtainable string, which is why it stayed green.

16. **The public edit-history collections carry no `nsfw=`, `facets=` or `refs=`** (D35b), and `entity_id=` without `object=` is 422 `INCONSISTENT_WITH` — entity ids are only unique within a family. The nsfw omission is deliberate: the two mirror crons read the collection ascending by id, and a visibility filter would silently skip rows the watermark has already passed.

17. **`edit_image` is not the `Image` type** (D35c) — it omits `source`, because freshly uploaded bytes have no upstream provenance. `sexual` / `violence` are always `null` on the receipt. The multipart file part declares `contentType: application/octet-stream` rather than an `image/*` list: `mime/multipart.CreateFormFile` stamps every part with octet-stream, so a narrower list rejects the only client this face exists for.

18. **The revision list query does not select the `snapshot` column**, and `field_diff` does not repeat `field_type` / `diff_hint`. Both are size decisions with a contract consequence: rendering metadata for a key lives in `GET /v2/catalog/schemas/{object}`, one definition rather than one per diff entry.

19. **Collection query parameters are repeated per input struct, never embedded.** huma v2.38 does not walk anonymous embedded structs when collecting query parameters, so an embedded `collectionInput` is dropped from the spec *and* never bound — the endpoint still answers 200 with `cursor` / `limit` / `sort` silently ignored. Four pre-existing routes are affected and are **not** fixed here (D35e): `GET /v2/catalog/redirects`, `GET /v2/me/playtimes`, `GET /v2/me/proposals`, `GET /v2/catalog/calendar`.

20. **`summary` on the news write bodies declares `maxLength: 2000`, not 200.** huma validates the generated schema before the handler runs and `problem.fieldFromHuma` maps *every* schema failure to reason `INVALID_FORMAT`; 06 §7.1 requires an over-long summary to come back as `TOO_LONG`. The 200-rune ceiling is therefore enforced in the handler (and again by the `news_item_preview_len` CHECK), and the schema bound is only a guard against an unbounded body. The **response** `summary` keeps `maxLength: 200`, which is the real contract.

21. **`{"status":"published"}` is `422`, not `409`.** The PATCH body declares `enum: [withdrawn]`, so the schema refuses it before the handler. 06 §7.2 says publishing "does not exist as a transition on the write face" — a value outside the field's vocabulary is the more accurate refusal than a state-machine 409. Every transition that *is* expressible but illegal (edit after publish, withdraw while pending, anything on a rejected item) is `409 me/INVALID_STATE_TRANSITION`.

22. **`GET/PATCH /v2/me/news/{id}` on another publisher's item is `404 NOT_FOUND`, not `403 news/SOURCE_NOT_YOURS`.** `SOURCE_NOT_YOURS` names the `source` member of a create body, where the caller chose the name. Returning it for an id lookup would make `/v2/me/news/{id}` an existence oracle over every other publisher's item ids — the same reason 10 §3.5 refuses to distinguish "not yours" from "does not exist".

23. **`SOURCE_INACTIVE` gates create and pending text edits, not withdrawal.** Obligation 3 (03 §2.4) requires a published item to stay withdrawable; deactivating a source must not strand its live items on the public face.

24. **Native `external_id` is `api_` + 32 hex characters.** `(source_key, external_id)` is UNIQUE and `lane` is not part of it. 月幕 writes bare snowflake digits and galgame 批评 writes `cv<id>#<ordinal>`, so the prefix is disjoint from both; a publisher who is also the `publisher_uid` on a partner source therefore cannot mint a row that the next importer pass adopts and overwrites.

25. **An application key on `/v2/me/news` is `401 INVALID_CREDENTIAL`,** not `403 me/USER_IDENTITY_REQUIRED`. This is the pre-existing `userAuth` bind shared by every `/v2/me` route: a `nmk_`/`nm_` key is rejected at the middleware. `USER_IDENTITY_REQUIRED` is what a token that resolves to no uid gets.

26. **`lane` defaults to `news`** when the create body omits it. The column is NOT NULL with no default and the public read type does not expose lane at all, so a default is invisible downstream; an unknown value is `422` with reason `UNKNOWN_VALUE`.

27. **The news item ETag is `"n<id>.<updated_at in microseconds>"`,** not a body digest. The value must survive a round trip through `If-Match`, and every write path bumps `updated_at`. The record is always re-read from Postgres after a write so the token comes from stored microseconds, never from a Go-side timestamp.

28. **Withdrawn items are `404` on `GET /v2/news/{id}`, not `410 GONE`.** Obligation 3 (03 §2.4) asks for a tombstone on the detail face. `news/service/public.go` filters `status = published AND dead_at IS NULL` for both list and detail, so list disappearance is correct and pending items never leak — only the detail status code is wrong. Pre-existing on the read face; not touched by the write wave.

29. **Rate limits follow the key tier, not the client IP** (2026-08-26). GA shipped a per-IP `100/min + 10,000/day` limiter that ignored the credential entirely, so a server-side integrator forwarding its whole site's reads from one egress IP burned the daily quota in minutes — the forum measured one page view of its companies index spending the backend's whole minute. Now: a request carrying a resolved `nmk_` key is limited per key at the devapi tier policy (`free` 60/min + 50k/day, `trusted` 600/min + 1M/day, `internal` unlimited; per-key overrides win), identical to the v1 face's `mw.RateLimit`/`mw.Quota`. Keyless `/v2` paths (problems, vocabularies, stats, schemas, openapi.json, and any request that never authenticates) keep the old per-IP defaults. `RateLimit-Policy` and the `X-RateLimit-*` headers now report the effective per-key numbers; unlimited keys get no rate headers. Mechanically the limiter moved from `protocol.Middleware` (pre-auth) to a `protocol.RateLimit` middleware behind the auth stack — so unauthenticated 401s are no longer counted, and a 429 is never remembered by the `Idempotency-Key` replay store (it would replay the refusal for 24h past the window reset).

30. **Entity faces carry the v1 read data, include-gated** (2026-08-26). The GA entity repr was id + names only, which is why the forum's entity pages fell back to v1. Additions — `companies`: `include=aliases,logo,intros,links` (aliases/logo fill on every lane from the list row; intros/links need the label detail read and fill on the detail face and the ids=/refs= batch lane, which upgrades to per-id detail reads when those tokens are present — the cursor list lane accepts but does not carry them), plus `has_works=` restored from v1 to skip the ~35k zero-work registry rows; `characters`: `include=image,figure,traits` on the detail face (`character_trait` rows with per-character spoiler/lie); `tags`: `is_sexual` joins the basic shape (the SFW gate the forum lost); `series`: `work_count` joins the basic shape (detail computes it from the same nsfw-gated count the list already had — this also adds `work_count` to the **v1** series detail projection); `engines`: `description` + `aliases` join the basic shape. The works face's engines block now refs a `WorkEngineRef` schema (same four properties as before) so the engine entity could grow without the work block lying `description: ""` on every row.

31. **The rating histogram and spread ride the detail faces only** (2026-08-26). `dto.PublicRating` has carried `distribution` and `stats` since the source importers filled them, but `ratingsFrom` dropped both, so the v2 face showed one aggregate number where v1 drew a chart. `Rating` now carries `distribution` (a `RatingBucket[]` on the source-native scale, ascending and sparse) and `stats` (`RatingStats`, all four members nullable). The presence rule is v1's, not a new one: the work detail face and `works/{id}/ratings` share `publicDetailRatings`, the works-list block goes through `publicRatings`, which never selected them — so a list response carries neither key and a client must not page ratings out of the list. `RatingBucket.score` is a `number`, not an `integer`, because G8 requires one JSON type per property name and `Rating.score` is already a `number`; every published scale is integral, so the wire bytes are the same. The denominator caveat is copied onto the field doc rather than left in the v1 DTO: bangumi, dlsite and howlongtobeat bars sum to `vote_count`, erogamescape bars come from a separately synced reviews mirror and are their own denominator, vndb bars omit private-list votes and sum to at most `vote_count`.

32. **Work companies carry the logo; tag and series detail grow their v1 blocks** (2026-08-26). `PublicWorkLabel.logo_hash` was already on the wire and thrown away, so a work page could not draw the brand mark v1 drew — `WorkCompany.logo` now fills from it through the same `ImageURL` builder deviation 30 added for the company face, which is why the work mappers take a `logoURL func(string) string` instead of reaching for the service. `tags/{id}` takes `include=intros`; `is_machine` is always `false` there and the doc says why (`catalog_tag_intro` has no provenance column, so `false` records *unknown*). `series/{id}` takes `include=intros,refs` and `has_nsfw` joins the basic shape on **every** lane — detail, cursor list and `ids=` batch — because `workCountsWithNSFW` already returned it beside `work_count` and the list DTO already carried it. `has_nsfw` is not `work_count > 0` under `nsfw=true`: it counts the same live-claimed members through the r18 *display* gate and is never narrowed by `nsfw=`, so a series legitimately reports `has_nsfw: true` with `work_count: 1` while the r18 member itself stays hidden. Both include blocks are detail-face only; the list route shares `SeriesSpec`, so it accepts the tokens and answers without them.

33. **`credit-names/{id}` is the staff read surface, and `repr.Person` stays thin** (2026-08-26). v1's staff page reads the NAME face, and `GetCreditName` was already calling `Public.Name` — the whole record — while mapping five fields out of it. The detail face now carries `gender`, `birth_year`, `birth_month` and `birth_day` as person-level facts reached through the name's public person link (absent when unrecorded or the link is hidden; the three birth parts are independently fuzzy), plus `include=aliases,photo,siblings,intros,links,refs`. `siblings` are the same person's other publicly linked names as basic `CreditName` entries, each stamped with that person id — a sibling shares the person by definition, and the sub-entries would otherwise arrive with `person_id: null` off a face reached through the person. The credits sub-face adds `character_name` beside `character_id` from `PublicNameRole.Character`, which `CreditNameCredits` was dropping. `repr.Person` is deliberately **not** enriched: the person face is an identity row, the name face is what a staff page reads, and the staff reverse lookup stays `credit-names/{id}/credits` (the decision recorded under Stage 5 read).

34. **The spoiler ceiling is a v2 closed enum on exactly three faces** (2026-08-27). The catalog service has always taken a spoiler ceiling; v2 passed a hardcoded `0` at three call sites and declared no parameter, so `spoiler>0` rows were permanently unreachable. `GET /v2/catalog/works/{id}`, `GET /v2/catalog/works/{id}/tags` and `GET /v2/catalog/characters/{id}` now take `spoiler=none|minor|major` (default `none`), mapped to v1's 0/1/2 at the boundary. It is **not** v1's integer: v2 publishes the closed `spoiler` vocabulary already used by `WorkTag.spoiler`, `CharacterTrait.spoiler` and `Appearance.spoiler`, and an unknown value is `400 UNKNOWN_ENUM_VALUE` like every other closed enum. The parameter is declared on three **dedicated** input structs, not on the shared `ResourceIDInput` / `WorkSubInput` — those are shared by every entity detail route and all twelve work sub-faces, and putting it there would publish an accepted-but-ignored `spoiler` on `engines/{id}`, `works/{id}/covers` and the rest, which is the exact defect deviation 35 refuses to widen. The doc keeps v1's nuance: only the VNDB-derived vocabulary records a spoiler level, Bangumi/DLsite folksonomy publishes no spoiler concept and those rows read `none`, so the default is the safe ceiling rather than an empty answer. Implementation trap, recorded in code: huma reflects over input structs and skips any field whose `IsExported()` is false — an **embedded** field takes its name from its type, so embedding the (then-unexported) `resourceIDInput` silently dropped `id`, `nsfw`, `view`, `include` and `fields` from both the generated spec and request binding while the route still answered 200. Both shared input types are exported for that reason alone.

35. **The company graph keeps company-set `include` validation; only `logo` applies** (2026-08-27). `getCatalogCompanyGraph` validated `include=` against `collect.CompanySpec()` and then discarded the parsed tokens, so `include=logo` passed the gate and changed nothing — `CompanyGraphNode` had no logo field at all, while v1's graph node carried `logo_hash` + `logo_meta`. The node now carries `logo`, present only when `include=logo` and that node has a logo, built through the same `imageURL` + `imageFromPublicMeta` path as the company face (the graph DTO carries `logo_meta`, so a node with dimensions and thumbhash gets them; a node whose image service did not answer still gets url + hash). The include set is deliberately **not** narrowed to a logo-only vocabulary: `aliases`, `intros` and `links` are 2xx on this route today, and rejecting them would be a breaking change. They are accepted and answered without, which the route Description now says out loud.

36. **`lang` joins the three entity shapes v1 carries it on** (2026-08-27). v1 publishes `lang` — "BCP-47 language of display_name" — on company (`PublicLabel` / `PublicLabelDetail`), credit name (`PublicName`) and character (`PublicCharacter`). v2 carried only `latin` + `localized{}`, and `localized{}` is a translations map: the language of `display_name` itself cannot be reconstructed from it. `repr.Company`, `repr.CreditName` and `repr.Character` now carry `lang` as an always-present nullable string beside `latin`, `null` when unrecorded. Scope is exactly those three — tags, series and engines do not carry the axis in v1 and do not gain it here. It is filled on **every** lane that emits these shapes: detail, cursor list, `ids=` batch, `credit-names/{id}?include=siblings`, `persons/{id}/credit-names` and the voice entries of `characters/{id}/appearances`. Two lanes had no source for it and got a purely additive **v1 read-projection extension** rather than a null that lies (deviation 30's precedent): `dto.PublicLabelListItem` gains `lang` and `LabelsList` selects it — the only change visible on a v1 wire, and only on `docs/catalog/public-openapi.yaml` — and the v2-private `catsvc.EntityListRow` gains `Lang`, with `CharactersList`, `NamesList` and `PersonNames` selecting `lang`. `PersonsList` and `TraitsList` share that row type but keep their own `SELECT`: `catalog_person` and `catalog_character_trait` have no such column, and `repr.Person` / `repr.Trait` do not publish one.

37. **Person `links[]` also mints from the two exact anchors that are addressable pages** (2026-08-27). `personLinks` selected only person-level `link_kind=related` rows, so the page-addressable identity anchors — the ones a staff page actually wants to link — were unreachable on both wires. `link_kind=exact` now mints for exactly two sources whose person-granularity id **is** a stable public page: **vndb** (`^s[0-9]+$` → `https://vndb.org/{id}`, 5,173/5,173 person rows conform) and **bangumi** (`^[0-9]+$` → `https://bgm.tv/person/{id}`, 2,767/2,767 conform, covering 2,740 distinct persons). An id of the wrong shape is skipped, never guessed. `probable` still never mints, and the other two sources holding person exact anchors stay excluded on purpose: **dlsite** (2,566 rows — creater ids have no public page) and **erogamescape** (4,428 — `creater.php` is unverified). Coverage of credit names carrying at least one link: **9,336 → 11,330 of 120,697** once vndb mints, **11,384** with bangumi added. The additions are purely additive on both wires (v1 `PublicName.Links`, shape `{source,url}` unchanged; v2 `credit-names/{id}?include=links`), the existing `link_visibility` gate is inherited unchanged, and the merged set keeps the single `ORDER BY r.source_id, r.external_id`. Minting is **person-only**: `workLinks` and the label links keep the shared `relatedLinkURL` template table, which is untouched. The incident this closes, recorded in code: the forum minted vndb URLs from the refs on a **credit name**, where a vndb ref is a staff **alias** id stored as bare digits (`1`, `10`, `100`) — `https://vndb.org/1` is an unrelated VN, and it shipped as a wrong-person page. `Ref.external_id`'s doc now says out loud that a ref is an identity anchor at that entity's own granularity and not necessarily a page, and that browseable URLs come from `links`.

38. **The silent 100-row cap on every embedded block is removed** (2026-08-27). A `cap100` helper truncated *every* `include=` block on the work, entity and credit-name faces — about 38 call sites — with no parameter, no `has_more`, and no entry in this ledger. v1 has no cap anywhere, so the two faces disagreed silently, and the truncation was already lying on production data: works with more than 100 rows in one block — tags 632 (max 264), credit rows 541 (max 340), characters 51 (max 354), releases 3 (max 160), relations 2 (max 322), covers 1 (max 104) — plus 651 characters with more than 100 trait links (max 227). It surfaced only because deviation 34's spoiler unlock pushed work-detail tag counts past 100 and the forum noticed a page that stopped at exactly 100. Blocks now return every row the read produced; the paged sub-faces (`works/{id}/tags` and the rest) remain the bounded access path and are unchanged. Live fixtures carry a work with 105 tag rows and a character with 105 trait links, and the tests assert the block agrees with the sub-face paged total.

39. **Company `latin` is filled, with the deviation-36 v1 projection extension** (2026-08-27). `repr.Company` declared `latin` from wave 3 and no lane ever populated it — a null that lies, on a column 542 of 38,317 live labels record. The root cause sat on the v1 side: the wave-209 name-primitive doctrine promises `display_name` + `latin` (when recorded) + `localized{}` on every projection that carries an entity name, but the two label head faces never got the second field — `PublicLabel` even documents the render rule `localized[yourLocale] ?? display_name ?? latin` while not carrying `latin`. Exactly like deviation 36's `lang`: `PublicLabel` and `PublicLabelListItem` gain `latin,omitempty`, `LabelWorks`/`LabelsList` select the column, and v2 maps it on the detail, cursor-list and `ids=` batch lanes. The v1 wire change is purely additive and only on `docs/catalog/public-openapi.yaml`. The deliberately thin projections stay thin: `WorkCompany` (the work-detail block), label `relations`, `via_label` and graph nodes never declared `latin` and do not gain it here.

40. **`/v2/me/playtimes` honors `include_total`** (2026-08-27). The lane parsed `include_total` like every other collection (bogus values 400) and then answered `"total": 0` unconditionally — the same accepted-then-ignored shape as deviation 10's works gap, found while fixing it. The cursor lane now counts the bearer's rows (all of them — the count ignores the `updated_at` cursor, deviation 10's count-before-cursor rule) and the `work_ids=` batch arm publishes the matched count, 0 on all-miss, instead of discarding the caller's flag.

41. **The playtime cursor could loop forever** (2026-08-27). Found by deviation 40's page-crawl test, which hung the suite at the 600s package timeout — twice. The `/v2/me/playtimes` cursor was the last row's `updated_at` in `time.RFC3339`, which truncates to whole seconds, and the next page selected `updated_at > cursor`: any two rows updated within the same second re-included the boundary row on every page, so a small-limit crawl never terminated and adjacent pages duplicated rows even when it did. The cursor is now `RFC3339Nano|work_id` — nanosecond precision plus a `work_id` tiebreak against the new `updated_at ASC, work_id ASC` order (`work_id` is unique per bearer) — and a bare-time cursor from before this change still parses, with the old semantics. The v1 `/playtime/mine` lane shares `ListMine` but keeps its strictly-after `updated_since` watermark semantics untouched: the tiebreak arm only engages when a work id is present, and v1 never passes one.

42. **`/v2/catalog/calendar` takes `content_limit=` and `olang=`** (2026-08-27). The v1 calendar validates both (`bogus` → 400) and feeds them into `CalendarFilter`; v2 declared neither, so `content_limit=bogus` answered 200 and an SFW reader got the unfiltered month — the third face to regress the editorial display gate after deviations 30 (tags `is_sexual`) and 34 (spoiler). Both are now declared on the calendar input and wired: `content_limit` through the same `closedCSV` the works lane uses, `olang` through a calendar-local parser because the defaults differ — absent `olang` on the calendar means the **home population, ja plus zh** (v1's zero `PublicOLang`, which v2 was already inheriting by passing the zero filter), while the works lane defaults to all languages; `olang=all` opts out.

43. **The calendar answers a `meta` navigation block** (2026-08-27). v1 serves `{today, min_month, max_month, has_prev, has_next}` off `CalendarBounds`; v2 returned a bare list, so a month navigator had no source for its arrows and the forum's month switcher died. The calendar output is now `CalendarList` — the list envelope plus an always-present `meta`. `today` (Asia/Tokyo) is always filled; the four navigation fields belong to the dated month window only and are `null` on the year-only and undated windows, exactly v1's shape: bounds computed under the same nsfw/olang/content_limit population, `min_month`/`max_month` null when that population has no dated release, `has_prev`/`has_next` false rather than null in that case.

44. **Work-detail roster rows carry `voices` and `identity`; covers carry `cover_kind`** (2026-08-27). `PublicRosterCharacter` has always carried both: the voice credits joined per character, and the opaque `roster:<character_id>` row identity a `catalog.work.roster.suppressed` proposal echoes back. v2's `WorkCharacter` mapped neither, so every character card lost its voice actors and no v2 client could file a roster suppression — the write engine accepts the patch, but the identity was published nowhere on /v2. `voices` maps to the same thin `CreditName` entries the appearances face already uses; `identity` is `omitempty` because a character reached only through a voice credit has no roster row to suppress (that arm reads `roster_role: unknown`, `spoiler: none`). `PublicCover.kind` — the upstream cover class from the pinning ladder (`main`, `dig`, `pkgfront`, …) — joins `Cover` as **`cover_kind`**: gate G8 forbids a bare `kind` property, so the field follows `title_kind`/`release_kind`/`tag_kind`. This reverses a recorded decision — `TestCoverHasRowIDNotKind` refused the field “until that vocabulary is closed” — because the values are upstream-derived and will never close; it ships as an explicitly **open** vocabulary under the `source`/`platform` convention (“must not be used as a discriminant”), empty when unrecorded.

45. **The company rollup lane names the intermediate imprint: `via_company`** (2026-08-27). On `company_id=&company_rollup=true`, v1 stamps each row reached through a one-hop imprint/subsidiary with `via_label` — the service computed it for v2 too (the zero `PublicFields` wants everything) and the v2 mapper threw it away, so a company page could not split its own works from its imprints' and the forum's own/imprint counts collapsed to N/0. `repr.Work` gains `via_company,omitempty`, a slim `{object, id, display_name, localized}` company stub, present only on rollup rows attributed through the hop and absent on direct attributions.

46. **`/v2/catalog/tags` honors `has_works=`** (2026-08-27). Deviation 30 restored `has_works=` on the companies lane and left the tags lane behind, though v1 carries it on both and `TagsListFilter.HasWorks` already existed — so the flag was silently ignored (the huma input never declared it) and a tag index drowned in zero-work rows. Declared and wired exactly like companies: `parse.Bool` (bogus → 400), same nsfw-gated `work_count > 0` predicate, total converges with the filter.

47. **User-token traffic is rate-limited per user, not per IP** (2026-08-28). Deviation 29 moved keyed traffic onto per-key tier limits and left "any request that never authenticates" on the per-IP default — but a user access token *does* authenticate and still resolved no limiter identity, so the whole `/v2/me` and `/v2/moderation` plane fell into the anonymous bucket. A first-party backend relays every logged-in user's call from one egress IP: the forum's entire user plane shared a single `100/min + 10k/day` bucket and answered 429 `QUOTA_EXCEEDED` from 2026-08-27T22:46Z, re-tripping after each UTC-midnight reset (`v2:quota:<forum egress IP>` measured 17,848 by 07:35Z the next day). Now `credentialLimitIdentity` falls through to the authenticated uid — key `u<uid>`, the default `100/min + 10,000/day` per user. Anonymous keyless paths (problems, vocabularies, stats, schemas, openapi.json) keep the per-IP default; an application key still wins over a uid when both are present.

## Stage 6 write

| Route | Bind |
|---|---|
| `POST /v2/me/playtimes` | 207 list of playtime-or-problem items |
| `POST /v2/me/claims` | `work_id` → `Act(claim)`; else `refs` → `LookupEntityID` then claim or mint; else `site_work_id` and/or `field_values` → mint. Every mint is one `SubmitWork` call, `Trusted` from `catalog.edit.trusted` (wave R4) |
| `GET/PATCH /v2/me/claims/{id}` | `{id}` is catalog work id. PATCH `{state: live\|pending\|withdrawn}` + If-Match, one `Act` call each (publish / submit / withdraw) |
| `POST /v2/me/edit-images` | multipart `preset` + `file`; `cover`/`screenshot` map to the image service's `catalog_cover`/`catalog_screenshot` |
| `GET/POST /v2/me/proposals` | `editing.Engine` |
| `GET/PATCH /v2/me/proposals/{id}` | PATCH withdraw or amend; If-Match |
| `POST /v2/me/proposals/{id}/amendments` | `AmendProposal` + If-Match |
| `GET /v2/moderation/claims/{id}` | Site-fenced `ClaimByWorkID` |
| `POST /v2/moderation/claims/{id}/decisions` | approve/decline/ban/unban + If-Match; requires `catalog.claim.review` |
| `GET /v2/moderation/proposals` | open proposals, `SITE_NOT_BOUND` without a catalog site |
| `GET /v2/moderation/proposals/{id}` | site-fenced; `include=patch` adds `patch` + `effective_patch`; ETag feeds its own decision |
| `POST /v2/moderation/proposals/{id}/decisions` | merge/decline + If-Match |
| `GET /v2/catalog/revisions` `/{id}` | public, application key; `sort=recorded_asc` is the watermark shape; `include=diff` + `diff_base=` |
| `GET /v2/catalog/proposals` `/{id}` | public, application key; no `patch`, no `decision_note`; `include=amendments` |
| `POST /v2/moderation/reverts` | `revision_id` loads `edit_revision` then `Revert` |
| `GET /v2/moderation/snapshots/{object}/{id}` | `CurrentSnapshot` |
| `GET /v2/me/news` | Items under `news_source.publisher_uid = caller`, pending included. Keyset on `news_item.id` DESC |
| `POST /v2/me/news` | `SubmissionService.Create` → always `status = pending`. 201 + `Location` + full body + `ETag`. `Idempotency-Key` via the shared POST middleware |
| `GET /v2/me/news/{id}` | `{id}` is the news item id; carries the `If-Match` ETag |
| `PATCH /v2/me/news/{id}` | Pending: edits `title`/`summary`/`source_url`/`banner_hash`/`work_ids`. Published: only `{"status":"withdrawn"}`, `If-Match` mandatory (428 without) |

`content_limit` on claim POST is not stored (no column on the claim row). Omit it.

News write specifics:

- **Zero schema change.** `upstream_category` and `banner_origin_url` are importer-only and are written as `''`; `news_item_work.confidence` is `0` (manual).
- **Re-moderation is inherited, not rebuilt.** `newsmoderate.Runner` selects `status = pending`, recomputes `NewsItem.Fingerprint()` (title + preview + lane) per row, and re-scores whenever no settled verdict exists for that exact digest. A pending text edit therefore re-enters the machine queue with no new code; a `source_url` / `banner_hash` / `work_ids` edit correctly does not.
- **`banner_hash` is a format check only** (`^[0-9a-f]{64}$`, the image service's sha256 hex). No call to the image service, so a syntactically valid hash for bytes that do not exist is accepted and renders as a broken banner in the moderation queue.
- **Refping tolerates native rows.** `news/imagerefs` collects `banner_hash <> ''` and never reads `banner_origin_url`, so an API-submitted banner with an empty origin is inside the daily sweep. ⚠️ The sweep authenticates as the **news** image client and is site-scoped: bytes a publisher uploaded under a different site's client will `not_found` on every ping and rot at the image service's TTL.

## Stage 7–8 (this repo)

- **MCP** — `cmd/mcp` loads `GET /v2/catalog/openapi.json` (or `KUN_MCP_OPENAPI_PATH`) and registers one read-only tool per GET on `/v2/catalog`, `/v2/news`, `/v2/problems`, `/v2/vocabularies`. Tool name = operationId. `nmk_` keys only. G10 compares tool params ⊇ HTTP query/path params.
- **SDK check** — `internal/platform/apiv2/sdk` Go client compiles and hits problems/vocabularies/works. TypeScript twin is the portal docs-model + explore relay.
- **Portal** — `apps/developer` documents the v2 face from `docs/catalog/v2-openapi.yaml`, HTML problem pages at `/problems/{domain}/{kebab}`, vocabularies, design principles. Explore and landing relay `/v2`.
- **Keys** — `/v2` rejects `nm_live_` / `nm_test_`. Mint and rotate issue `nmk_live_` / `nmk_test_` with CRC32 for every tier (GA); rotation upgrades a v1 key to `nmk_`. v1 still accepts both generations (malformed `nmk_` rejected offline).
- **Edge** — `docker-compose.prod.yml` routes `Host(api.nextmoe.dev) && PathPrefix(/v2)` to catalog. Landing this branch on `main` deploys that router with the catalog image. The developer portal and MCP compose projects stay manual.

## Stage 9–10

- **9 GA — declared 2026-08-25.** Landed with it: `nmk_` minting opened to every dev tier (rotation of a v1 key returns `nmk_`), the portal preview banner and every preview qualifier dropped, `info.version 2.0.0` / `x-stability: stable`, and `docs/catalog/v2-openapi.yaml` added to the `specs=` list in `spec-breaking.yml` (G12). That last one is the whole gate: the file was already in the workflow's trigger `paths` but not in `specs=`, so oasdiff had never actually diffed it — a v2 spec that looked checked and was not. Default masks and the existing error-code registry entries are frozen from this date. Still open: notify the 17 owners.
- **10 v1 410** — Deprecation/Sunset counted from GA (2026-08-25), brownout, then `/v1/**` 410.

## Tests that exist

- Spec-driven: `TestContractHitsEverySpecOperation` hits every registered op; undeclared status fails.
- Gates G2–G16 on the generated OpenAPI document.
- Per-bind unit tests for mapping, unbound 503, auth 401, NSFW, cursor, schema `field_type`.
- Live DB: `handler/live_*_test.go` (requires track `TEST_DATABASE_DSN`). One 200 GET per bound read, write happy paths, then a spec walk against the live app.
- News write: `handler/live_news_test.go` over HTTP (source grant 403/422, POST→pending with `Location`/`ETag`/`no-store`, field-level 422, `Idempotency-Key` replay, pending edit, 428/412/200 on withdrawal, rejected terminal, cross-publisher 404) and `news/service/submission_test.go` at the service layer, which drives the **real** `newsmoderate.Runner` to prove a text edit re-enters the machine queue and a metadata edit does not.

Search 200 paths need Meilisearch; with it unbound those ops return declared 503. The news tables are created in the same track database by `newsmigrate.Run`, so the news read and write faces are bound in the live tests.

When a test database is assigned, run:

`GOMAXPROCS=8 go test -count=1 -p 1 ./internal/platform/apiv2/... ./internal/platform/news/...`

A DB-backed package that "passes" in well under a second ran nothing: read the `-v` PASS counts, not the `ok` line.

## Wave R1 — v2 fills for the v1 retirement (2026-08-27)

Four new operations, and the last v1-only reads that had no v2 answer are gone.

| Operation | Notes |
| --- | --- |
| `GET /v2/catalog/claim-events` | The claim lifecycle log. `sort=recorded_asc` is the watermark walk a mirror or a reward cron reads; `site=`/`actor_uid=`/`work_id=` narrow it, `ids=` is the batch lane. Needs `claim_events:read` **on top of** `catalog:read` |
| `DELETE /v2/me/claims/{id}` | Deletes a draft the caller owns. A live or pending claim must be withdrawn to draft first (`PATCH state=withdrawn`). Soft-deletes the `catalog_work` row and writes **no** claim event, matching v1's executor. Keyed on owner uid, not site |
| `GET /v2/store/purchase-links/{product_id}` | v1 `/v1/store/purchase-links/:product_id` with RFC 9457 problems and no envelope |
| `GET /v2/store/stats` | v1 `/v1/store/me/stats`. The bearer *application* is the subject, so the path drops the `me/` segment that only made sense next to a user face |

`claim_events:read` is **operator-granted, never self-service** (`selfServiceScopes`
stays `[catalog:read, store:read]`): events carry decline reasons and the
moderator uid behind every decision, which is a different disclosure than the
registry rows `catalog:read` buys. The gate is two-stage in `catalogAuth` —
`catalog:read` first, then the extra scope for that one path — so a key without
it gets `SCOPE_REQUIRED` naming the missing scope rather than a bare 403.

Read parity and hardening in the same wave:

- **`/v2/me/claims` returns what the service always carried.** `claimRecordFrom`
  dropped 9 of `UserClaimItem`'s 13 fields; `site`, `product_work_id`,
  `last_event` (id / from_state / to_state / reason / actor_uid / created_at),
  `first_acted_at` and `acted_count` are now published. The last three come from
  the actor aggregate, so they are present only when the row carries a last
  event — the moderation queue rows have no aggregate and omit the block.
- **`kind=` and filters on `/v2/me/claims`.** `submitted` (default) keeps works
  the bearer owns, `audited` the ones they reviewed but do not own, `all`
  everything they touched. `claim_state=` takes all six states here — this is
  the bearer's own history, not the public registry — and `site=` also scopes
  `first_acted_at`/`acted_count`.
- **`claim_state=` on `/v2/catalog/works` narrows to `none,live,draft`.**
  `pending`/`declined`/`hidden` are moderation-workflow states; v1 kept its
  pending queue behind a moderation gate and the first v2 cut left all six open
  to any `catalog:read` key. The queue is `/v2/moderation/claims`.
- **`owner_uid=` on `/v2/catalog/works`** requires `site=` (owner uids are the
  claiming site's own user ids, so they are ambiguous alone → 422
  `INCONSISTENT_WITH`) and is refused on the search lane with
  `MUTUALLY_EXCLUSIVE_PARAMETERS` — the search index carries no claim owner.
- **The moderation claim reads demand `catalog.claim.review`.** `ListModerationClaims`
  and `GetModerationClaim` only required a site-bound user token; only
  `DecideClaim` checked the permission. A plain user could therefore read the
  whole pending queue including decline reasons.
- **`POST /v2/me/claims` accepts `{site_work_id, display_name}`.** That payload
  used to fall through to the "work_id or refs is required" 422 — the only
  anchored-mint shape v1 offered had no v2 equivalent. An existing
  `(site, product_work_id)` anchor still returns `ALREADY_EXISTS` (409) from
  `SubmitWork`, unchanged.

Shape notes worth keeping:

- `store_stat.link_kind`, not `kind`: **G8 forbids a bare `kind` property**, the
  same rule that produced `image.cover_kind`.
- `store_stats.from_date`/`to_date`, not `from`/`to`: G8 wants one shape per
  property name and `FieldDiff.from`/`to` are untyped (`any`). The query
  parameters are still `from=`/`to=` — parameters are not properties.
- `GET /v2/store/purchase-links/{product_id}` declares 404 because **G4** requires
  it of any path-parameter operation. Nothing mints one: `product_id` is a
  DLsite number this service never resolves against a catalog, so an
  unknown-but-well-formed id still gets a link.
- Both store operations always send `Cache-Control: private`. Every response is
  keyed to the calling application; a shared cache holding one site's short
  links and serving them to another hands the clicks to the wrong site.
- One `store/service.Service` instance is shared by the v1 and v2 faces. A
  second one would mint a second alias for the same `(client, product)` pair and
  split the click count that settlement reads.
- **`GET /v2/store/stats` deliberately answers without the shortener**: only the
  purchase-links op requires `Configured()`. Stats reads the database and never
  mints, and v1's `MyStats` only required the service to exist — gating both ops
  on `Configured()` would take the stats read away from a deployment missing the
  shortener credentials, which is exactly the regression this wave exists to
  avoid. An unbound service (`Catalog.Store == nil`) is the one case both refuse.

Two new problem codes in a new `store` domain: `STORE_QUOTA_EXCEEDED` (403) and
`STORE_LINK_UNAVAILABLE` (502). The 502 is deliberate and has no fallback to a
bare affiliate URL — a raw aff link bypasses the click counter, which is the one
thing the de-duplication promise to DLsite rests on.

Edge and portal: `/v2/store` needs no new Traefik router — `infra-v2-pub` already
matches `PathPrefix(/v2)`. The portal relay allowlist gained `v2/store/`.
oasdiff against the GA baseline reports **0 error, 8 warning** (all
`response-property-enum-value-added`, from `claim.state` gaining `none` and
`problem_type.domain` gaining `store`), so no entry was added to
`v2-openapi-breaking-ignore.txt`.

## Wave R3 — the total v1 teardown (2026-08-27)

Wave R1 filled the last capability gaps; this wave deleted what they replaced.
The catalog binary no longer serves any v1 face.

Removed, by family:

- **public `/v1/catalog`** — the fiber group in `cmd/catalog`, every
  `public_*.go` handler, the spec twins `public_huma.go` +
  `public_huma_work_subresources.go`, the `/v1/catalog/openapi.json` route and
  `docs/catalog/public-openapi.yaml` (+ its breaking-ignore).
- **`/v1/playtime`** — the mount, the gate plumbing and `user_playtime.go`. The
  `UserPlaytimeService` stays: `/v2/me/playtimes` rides it.
- **`/v1/news`** — the group, `news/handler/public.go`'s `PublicHandler`,
  `SetupNewsPublicSpec` and `docs/news/public-openapi.yaml`. The parsing helpers
  the admin queue shares moved to `news/handler/params.go`; the news services and
  `/api/v1/admin/news` are untouched.
- **`/v1/store`** — the group, `store/handler/public.go`, its spec twin and
  `docs/store/public-openapi.yaml`. The store service, `dev.go` (the portal's
  owner-usage panel) and `/v2/store` are untouched, and both v2 store ops still
  ride the single shared `store/service.Service` instance.
- **S2S `/api/v1/catalog`** — `S2SAuth`/`authenticateBasic`/`enforceSiteBinding`,
  `s2s.go`, `read.go`, `edit.go`, `lifecycle.go`, `user_claims.go`, the huma API
  build, the root `/openapi.json` dump and `docs/catalog/openapi.yaml`.
- **user `/api/v1/user/catalog`** — `user.go`, `user_claims_face.go`,
  `user_edit.go`, `user_edit_reads.go`, `user_read.go`, `user_cover_votes.go`
  and `edit_images.go`.

Six prefixes now answer `410 Gone` from
`internal/platform/catalog/handler/retired.go`: `/v1/catalog`, `/v1/news`,
`/v1/store`, `/v1/playtime`, `/api/v1/catalog`, `/api/v1/user/catalog`. The body
is the house envelope with `code: 11`, the header is
`Link: <https://api.nextmoe.dev/v2>; rel="successor-version"`, and neither
`Deprecation` nor `Sunset` is sent — the face is gone, not deprecating. They are
mounted **after** the admin groups and after the v2 setup, because fiber matches
in registration order; `retired_test.go` proves the non-capture rather than
assuming it (`/api/v1/admin/catalog/*` reaches its permission gate, `/v2/catalog/works`
and `/v2/store/stats` still answer 401 problem+json).

The galgame tombstone (`internal/galgameapp/publicgone.go`) named `/v1/catalog`
as its successor, which would have become a tombstone pointing at a tombstone.
It now points at `/v2`.

Two follow-on removals the teardown forced:

- `catalog/service.WorkService` (and its `deriveAnchorRating`) had exactly one
  non-test caller, the S2S claim op. v2 mints through
  `ClaimLifecycleService.SubmitWork`, so the type is gone;
  `DeriveContentRating` and the shared `sourceRefR18` stay.
- devapi usage metering was wired into the v1 fiber groups only. Deleting them
  would have removed every writer of `developer_api_usage` and every
  `TouchLastUsed` call, leaving the portal's usage panel and each key's
  "last used" permanently empty with nothing failing. The recorder is now a
  `/v2` prefix middleware recording under the face name `v2`.

`nm_` v1 application keys were left in place as inert rows: nothing reads them
any more (only `/v1` accepted the prefix, and `/v2` refuses it with
`INVALID_CREDENTIAL`), and deleting them would take the owner's key list and
usage history with them. **This wave carries no migration** — no model, no DDL.

Plumbing that went with the specs: the `-catalog`, `-catalog-public`,
`-news-public` and `-store-public` arms of `cmd/gen-openapi`; the matching regen
lines and diff paths in `test.yml`; the four retired specs in
`spec-breaking.yml`; the catalog S2S arm of `openapi-types.yml` together with
`apps/web/shared/types/generated/catalog-api.ts` (apps/web never imported it);
the five v1 faces in `apps/developer/scripts/faces.mjs` with their `FACE_GROUPS`
table, `EDIT_EXCLUDED_OPERATION_IDS`, the per-operation auth override table and
their `EXPECTED_OPERATION_COUNTS` entries; the `v1/store/` entry in the portal
relay allowlist; and the generated per-operation Markdown twins under
`apps/developer/public/docs/{catalog,edit,news,playtime,store}`.
`docs/news/` and `docs/store/` held nothing else and are gone as directories.

Traefik/compose were deliberately **left untouched**: the v1 routers point at the
same catalog service that now answers 410 on those prefixes, so removing them
would turn a documented 410 into a 404 that names no successor.

## Wave R4 — `POST /v2/me/claims` carries the wizard's field map (2026-08-27)

R1 filled the capability gaps and R3 deleted v1, but one caller was left without
a lane: moyu's publish wizard. Its v1 call `POST /api/v1/user/catalog/works/submit`
handed an **editing-engine field map** straight to `SubmitWork` — multi-language
official titles, lang-less aliases, `olang`, an explicit content rating,
`display_nsfw` and a zh-Hans intro — and `POST /v2/me/claims` had nowhere to put
one. Every v2 mint was born with a display name and links and nothing else. This
wave adds an optional `field_values` object to the request body: additive, no new
operation (88 stays 88), no response change, **no migration**.

**The property is `field_values`, not v1's `fields`.** Gate G8 wants one shape
per property name, and `object_schema.fields` is already an array of
`schema_field` descriptors — `fields` as an object fails `TestG7toG16`, the same
rule that produced `cover_kind` and `from_date`/`to_date`. `field_values` is not
a coined name: `snapshot.field_values` already carries exactly this map, so
`POST /v2/me/claims {field_values}` mints the work that
`GET /v2/moderation/snapshots/work/{id}` answers with the same key. (`patch`,
the proposal faces' spelling of the same map, was the other candidate; it reads
as an edit to something that exists.)

`field_values` is a bare `map[string]any` in the schema on purpose. The accepted
keys are the service's whitelist (`editspec.submissionFields`), and duplicating
it in the huma input would be a second copy to drift. An unrecognised key answers
`VALIDATION_FAILED` with pointer `/field_values/<key>`, from
`editspec.SubmissionFieldError` — which `claimWriteErr` had never mapped, so
before this wave it would have escaped as a 500.

The lanes:

| Body | Result |
|---|---|
| `work_id` + `field_values` | **422** `VALIDATION_FAILED`, pointer `/field_values` |
| `refs` that resolve to a work, **with** `field_values` | **409** `ALREADY_EXISTS`, naming the anchor and that work's id |
| `refs` that resolve to a work, no `field_values` | claims it — unchanged |
| `refs` with no match, `field_values` | mints; the ref-derived URLs are unioned into `catalog.work.links` |
| `site_work_id` + `field_values` | mints anchored to the site's own id |
| `field_values` alone | mints — moyu's exact shape |

Decisions behind that table:

- **A ref that already resolves is 409, not a silent claim.** Without
  `field_values` the old behaviour stands. With them the caller asserted content
  for a work they believe is new, so claiming the match would answer 201 while
  dropping the whole map on the floor. The last-resort problem now reads
  "work_id, refs, site_work_id, or field_values is required."
- **The ref anchors survive a caller who also sends links.** The URLs derived
  from `refs` are unioned into `catalog.work.links` rather than replacing or
  being replaced by the caller's list, deduped on the exact URL string. They are
  what `SubmissionAnchorsOf` turns into the identity anchors a later submission
  is recognised by, so losing them to a caller-supplied `links` would make the
  mint unrecognisable to its own retry.
- **`display_name` resolution.** Top-level `display_name` wins and is written
  into the map; otherwise `field_values["catalog.work.display_name"]` must be present;
  otherwise the existing `/display_name` required problem. The map's display name
  is only a **seed**: inside the same transaction `applyTitles` runs after
  `olang` and rewrites `catalog_work.display_name` to the official title in the
  work's `olang`. That is `ApplyWorkFields`'s standing behaviour and was v1's
  too — the wizard sends the zh-Hans name first with `olang: ja`, and the work is
  born under its Japanese title.
- **An explicit `content_rating` is honored.** When
  `field_values["catalog.work.content_rating"]` is present `DeriveContentRating` is
  skipped entirely: the field goes through editspec and stamps curated
  provenance, where the derived value is written straight to the column with no
  stamp so the weekly releasemeta lane can still correct it. When it is absent
  the derivation is exactly what it was.
- **The trusted fast lane came back.** v1 computed
  `catperm.Resolver.Can(roles, EditTrusted)` and passed it as
  `SubmitWorkParams.Trusted`, which mints `live` instead of `pending`; v2's
  `CreateClaim` never set it, so an editor holding `catalog.edit.trusted` lost
  the privilege by moving to /v2. It is now set on every mint lane. v1 also
  required `!isThirdPartyClient`; that half is **not** reproduced — see below.
- **`released` is deliberately still absent.** `SubmitWorkParams.Released` stays
  zero from /v2, so `ErrSubmitInvalidDate` is unreachable here and is not mapped.
  No caller ever sent it.

On the third-party half of v1's trusted condition: v2 has no notion of a
third-party client anywhere in `apiv2`, and site binding is **not** structurally
first-party-only. `ctxSite` is filled from `Options.LookupSite`, which in
`cmd/catalog` is `clientRepo.FindByClientID(clientID).CatalogSite` with no
first-party check, while third-party-ness is the independent column
`oauth_clients.owner_user_id`. Nothing in the schema forbids a row from carrying
both. What does hold is that **no code path can produce that row**: the only
writer of `owner_user_id` is `devapi.SelfServiceService.CreateApp`, which never
sets `catalog_site`; `devapi.AdminService.UpdateAppConfig` cannot write
`catalog_site`; and `site/service`'s first-party client creation does not set it
either. A third-party app with a catalog site can only be made by an operator
editing the column by hand. The guard would also not be a privilege boundary on
/v2: `patchMyClaim` lets a claim's owner move `pending → withdrawn → live`
(publish requires ownership, not a permission), so the same actor reaches `live`
in two more calls with or without `Trusted`. Adding the check would mean
widening `Options.LookupSite` to carry the client's owner, for a fence that is
already open next door.
