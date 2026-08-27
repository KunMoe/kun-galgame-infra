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

10. **Works list `include=` hydrates the list-capable subset** (`titles`, `intros`, `companies`, `ratings`, `covers` slots, `refs`). Other FULL_SET tokens stay on detail / sub-resources. SQL `include_total` is omitted (search path returns Meili `total`). Facets on the SQL browse path return empty buckets; named facet counts come from Meili.

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

31. **The rating histogram and spread ride the detail faces only** (2026-08-26). `dto.PublicRating` has carried `distribution` and `stats` since the source importers filled them, but `ratingsFrom` dropped both, so the v2 face showed one aggregate number where v1 drew a chart. `Rating` now carries `distribution` (a `RatingBucket[]` on the source-native scale, ascending and sparse) and `stats` (`RatingStats`, all four members nullable). The presence rule is v1's, not a new one: the work detail face and `works/{id}/ratings` share `publicDetailRatings`, the works-list block goes through `publicRatings`, which never selected them — so a list response carries neither key and a client must not page ratings out of the list. `RatingBucket.score` is a `number`, not an `integer`, because G8 requires one JSON type per property name and `Rating.score` is already a `number`; every published scale is integral, so the wire bytes are the same. The denominator caveat is copied onto the field doc rather than left in the v1 DTO: bangumi and dlsite bars sum to `vote_count`, erogamescape bars come from a separately synced reviews mirror and are their own denominator, vndb bars omit private-list votes and sum to at most `vote_count`.

32. **Work companies carry the logo; tag and series detail grow their v1 blocks** (2026-08-26). `PublicWorkLabel.logo_hash` was already on the wire and thrown away, so a work page could not draw the brand mark v1 drew — `WorkCompany.logo` now fills from it through the same `ImageURL` builder deviation 30 added for the company face, which is why the work mappers take a `logoURL func(string) string` instead of reaching for the service. `tags/{id}` takes `include=intros`; `is_machine` is always `false` there and the doc says why (`catalog_tag_intro` has no provenance column, so `false` records *unknown*). `series/{id}` takes `include=intros,refs` and `has_nsfw` joins the basic shape on **every** lane — detail, cursor list and `ids=` batch — because `workCountsWithNSFW` already returned it beside `work_count` and the list DTO already carried it. `has_nsfw` is not `work_count > 0` under `nsfw=true`: it counts the same live-claimed members through the r18 *display* gate and is never narrowed by `nsfw=`, so a series legitimately reports `has_nsfw: true` with `work_count: 1` while the r18 member itself stays hidden. Both include blocks are detail-face only; the list route shares `SeriesSpec`, so it accepts the tokens and answers without them.

33. **`credit-names/{id}` is the staff read surface, and `repr.Person` stays thin** (2026-08-26). v1's staff page reads the NAME face, and `GetCreditName` was already calling `Public.Name` — the whole record — while mapping five fields out of it. The detail face now carries `gender`, `birth_year`, `birth_month` and `birth_day` as person-level facts reached through the name's public person link (absent when unrecorded or the link is hidden; the three birth parts are independently fuzzy), plus `include=aliases,photo,siblings,intros,links,refs`. `siblings` are the same person's other publicly linked names as basic `CreditName` entries, each stamped with that person id — a sibling shares the person by definition, and the sub-entries would otherwise arrive with `person_id: null` off a face reached through the person. The credits sub-face adds `character_name` beside `character_id` from `PublicNameRole.Character`, which `CreditNameCredits` was dropping. `repr.Person` is deliberately **not** enriched: the person face is an identity row, the name face is what a staff page reads, and the staff reverse lookup stays `credit-names/{id}/credits` (the decision recorded under Stage 5 read).

34. **The spoiler ceiling is a v2 closed enum on exactly three faces** (2026-08-27). The catalog service has always taken a spoiler ceiling; v2 passed a hardcoded `0` at three call sites and declared no parameter, so `spoiler>0` rows were permanently unreachable. `GET /v2/catalog/works/{id}`, `GET /v2/catalog/works/{id}/tags` and `GET /v2/catalog/characters/{id}` now take `spoiler=none|minor|major` (default `none`), mapped to v1's 0/1/2 at the boundary. It is **not** v1's integer: v2 publishes the closed `spoiler` vocabulary already used by `WorkTag.spoiler`, `CharacterTrait.spoiler` and `Appearance.spoiler`, and an unknown value is `400 UNKNOWN_ENUM_VALUE` like every other closed enum. The parameter is declared on three **dedicated** input structs, not on the shared `ResourceIDInput` / `WorkSubInput` — those are shared by every entity detail route and all twelve work sub-faces, and putting it there would publish an accepted-but-ignored `spoiler` on `engines/{id}`, `works/{id}/covers` and the rest, which is the exact defect deviation 35 refuses to widen. The doc keeps v1's nuance: only the VNDB-derived vocabulary records a spoiler level, Bangumi/DLsite folksonomy publishes no spoiler concept and those rows read `none`, so the default is the safe ceiling rather than an empty answer. Implementation trap, recorded in code: huma reflects over input structs and skips any field whose `IsExported()` is false — an **embedded** field takes its name from its type, so embedding the (then-unexported) `resourceIDInput` silently dropped `id`, `nsfw`, `view`, `include` and `fields` from both the generated spec and request binding while the route still answered 200. Both shared input types are exported for that reason alone.

35. **The company graph keeps company-set `include` validation; only `logo` applies** (2026-08-27). `getCatalogCompanyGraph` validated `include=` against `collect.CompanySpec()` and then discarded the parsed tokens, so `include=logo` passed the gate and changed nothing — `CompanyGraphNode` had no logo field at all, while v1's graph node carried `logo_hash` + `logo_meta`. The node now carries `logo`, present only when `include=logo` and that node has a logo, built through the same `imageURL` + `imageFromPublicMeta` path as the company face (the graph DTO carries `logo_meta`, so a node with dimensions and thumbhash gets them; a node whose image service did not answer still gets url + hash). The include set is deliberately **not** narrowed to a logo-only vocabulary: `aliases`, `intros` and `links` are 2xx on this route today, and rejecting them would be a breaking change. They are accepted and answered without, which the route Description now says out loud.

36. **`lang` joins the three entity shapes v1 carries it on** (2026-08-27). v1 publishes `lang` — "BCP-47 language of display_name" — on company (`PublicLabel` / `PublicLabelDetail`), credit name (`PublicName`) and character (`PublicCharacter`). v2 carried only `latin` + `localized{}`, and `localized{}` is a translations map: the language of `display_name` itself cannot be reconstructed from it. `repr.Company`, `repr.CreditName` and `repr.Character` now carry `lang` as an always-present nullable string beside `latin`, `null` when unrecorded. Scope is exactly those three — tags, series and engines do not carry the axis in v1 and do not gain it here. It is filled on **every** lane that emits these shapes: detail, cursor list, `ids=` batch, `credit-names/{id}?include=siblings`, `persons/{id}/credit-names` and the voice entries of `characters/{id}/appearances`. Two lanes had no source for it and got a purely additive **v1 read-projection extension** rather than a null that lies (deviation 30's precedent): `dto.PublicLabelListItem` gains `lang` and `LabelsList` selects it — the only change visible on a v1 wire, and only on `docs/catalog/public-openapi.yaml` — and the v2-private `catsvc.EntityListRow` gains `Lang`, with `CharactersList`, `NamesList` and `PersonNames` selecting `lang`. `PersonsList` and `TraitsList` share that row type but keep their own `SELECT`: `catalog_person` and `catalog_character_trait` have no such column, and `repr.Person` / `repr.Trait` do not publish one.

## Stage 6 write

| Route | Bind |
|---|---|
| `POST /v2/me/playtimes` | 207 list of playtime-or-problem items |
| `POST /v2/me/claims` | `work_id` → `Act(claim)`; else `refs` → `LookupEntityID` then claim or mint with `display_name` + links |
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
