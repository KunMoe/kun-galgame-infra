# v2 implementation notes (preview)

This page records **binds that differ from `refs/api-v2`**. It is not a substitute for the design spec. Code + `docs/catalog/v2-openapi.yaml` are the machine-readable contract.

Date: 2026-08-23. Branch: `v2-stage0`. No push (deploy is automatic).

## Done on this branch

- Protocol, problems, vocabularies, collection contract, repr types, CI gates G1–G16.
- Catalog read: works (list/detail/12 subs), companies+graph, tags, series, engines, releases, characters, credit-names, persons, traits, search, calendar, changes, redirects, stats, schemas/{object}, news.
- Character `view=full` carries D30 attributes (`gender`, `birthday` as `MM-DD`, measurements, `blood_type` as `a|b|ab|o`, `instance_of_id`). `description` / `extra` / `field_provenance` stay out, as D30.
- `/v2/me/playtimes` GET/PUT/DELETE and POST 207 batch; `/v2/me/cover-votes` GET/PUT/DELETE.
- `/v2/me/claims` list (keyset on last claim-event id)/create/get/withdraw; `/v2/me/proposals` list/create/get/patch/amend.
- `/v2/moderation/claims` queue + GET by id + decisions; `/v2/moderation/proposals` queue + decisions; reverts; snapshots. Queue and GET are site-fenced (`SITE_NOT_BOUND` when the token client has no catalog site).
- User token on `/v2/me` and `/v2/moderation`, not an application key; `private, no-store`. JWT `roles` are copied into the handler context so `HasPerm` matches v1.

## Spec deviations (with reason)

1. **`series` and `traits` `refs=`** — `catalog_external_ref` has no entity_type for series or trait (`constants.go` entity types stop at engine=8). Unknown refs go to `missing[]`. Not a new table.

2. **Character full fields use `omitempty`** — D30 says unrecorded is JSON `null`. `view=basic` must not emit those keys (default-thin). Unrecorded values on `view=full` are therefore omitted rather than `null`. Same projection trick as work `include=` blocks.

3. **Trait JSON uses `is_sexual`, not `sexual`** — G8: `Screenshot.sexual` is `*string` (`safe|suggestive|explicit`). A bool cannot share the property name. Work tags already use `is_sexual`.

4. **Cover vote is `up` only** — v1 stores one ballot per (user, work). There is no down column. `vote=down` is 422.

5. **Playtime GET empty is 404** — v1 returned 200 + null. v2 has no envelope; a missing row is `NOT_FOUND`.

6. **Playtime delete** — v1 had no DELETE. v2 DELETE removes `catalog_user_playtime` rows for that (user, work). Additive service method, existing table.

7. **`blood_type` public tokens are lowercase** (`a|b|ab|o`) per 04-representation. D30's table wrote `A|B|AB|O` as domain letters; the editing engine stores int16. Public JSON follows 04.

8. **Claim POST mint needs `display_name`** — D33. `SubmitWork` cannot mint without `catalog.work.display_name`. `refs` that miss `catalog_external_ref` become `catalog.work.links` URLs via the existing source URL templates. `work_id` and `refs` both absent is 422 even if `display_name` is present.

9. **Moderation claims/proposals lists are not keyset-paginated yet** — pending claims order by submitted event id and return one page. `/v2/me/claims` is keyset-paginated. The queues stay one page until `PendingClaims` grows a cursor.

## Stage 6 write

| Route | Bind |
|---|---|
| `POST /v2/me/playtimes` | 207 list of playtime-or-problem items |
| `POST /v2/me/claims` | `work_id` → `Act(claim)`; else `refs` → `LookupEntityID` then claim or mint with `display_name` + links |
| `GET/PATCH /v2/me/claims/{id}` | `{id}` is catalog work id. PATCH `{state:withdrawn}` + If-Match |
| `GET/POST /v2/me/proposals` | `editing.Engine` |
| `GET/PATCH /v2/me/proposals/{id}` | PATCH withdraw or amend; If-Match |
| `POST /v2/me/proposals/{id}/amendments` | `AmendProposal` + If-Match |
| `GET /v2/moderation/claims/{id}` | Site-fenced `ClaimByWorkID` |
| `POST /v2/moderation/claims/{id}/decisions` | approve/decline + If-Match |
| `GET /v2/moderation/proposals` | open proposals, `SITE_NOT_BOUND` without a catalog site |
| `POST /v2/moderation/proposals/{id}/decisions` | merge/decline + If-Match |
| `POST /v2/moderation/reverts` | `revision_id` loads `edit_revision` then `Revert` |
| `GET /v2/moderation/snapshots/{object}/{id}` | `CurrentSnapshot` |

`content_limit` on claim POST is not stored (no column on the claim row). Omit it.

## Stage 7–10 (not this repo's HTTP surface)

- **7 MCP / SDK** — generated from `v2-openapi.yaml` in kungal-docs / client repos.
- **8 preview portal, HTML problem catalog, design-principles page** — `developer.nextmoe.dev`, not `cmd/catalog`.
- **9 GA / 10 v1 410** — product announcement, consumer migration, Sunset headers. Not code-completeable from infra alone.

## Tests that exist

- Spec-driven: `TestContractHitsEverySpecOperation` hits every registered op; undeclared status fails.
- Gates G2–G16 on the generated OpenAPI document.
- Per-bind unit tests for mapping, unbound 503, auth 401, NSFW, cursor, schema `field_type`.
- Live DB: `handler/live_*_test.go` (requires track `TEST_DATABASE_DSN`). One 200 GET per bound read, write happy paths, then a spec walk against the live app.

News and search 200 paths need the news DB and Meilisearch. With those unbound, those ops return declared 503.

When a test database is assigned, run:

`GOMAXPROCS=8 go test -count=1 -p 1 ./internal/platform/apiv2/handler/ -run 'TestLive|TestContract|TestMe|TestClaims'`
