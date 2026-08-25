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
