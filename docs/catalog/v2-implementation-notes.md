# v2 implementation notes (preview)

This page records **binds that differ from `refs/api-v2`** and **Stage 6–8 work that is not mounted yet**. It is not a substitute for the design spec. Code + `docs/catalog/v2-openapi.yaml` are the machine-readable contract.

Date: 2026-08-23. Branch: `v2-stage0`. No push (deploy is automatic).

## Done on this branch

- Protocol, problems, vocabularies, collection contract, repr types, CI gates G1–G16.
- Catalog read: works (list/detail/12 subs), companies+graph, tags, series, engines, releases, characters, credit-names, persons, traits, search, calendar, changes, redirects, stats, schemas/{object}, news.
- Character `view=full` carries D30 attributes (`gender`, `birthday` as `MM-DD`, measurements, `blood_type` as `a|b|ab|o`, `instance_of_id`). `description` / `extra` / `field_provenance` stay out, as D30.
- `/v2/me/playtimes` GET/PUT/DELETE and `/v2/me/cover-votes` GET/PUT/DELETE.
- `/v2/me/claims` and `/v2/moderation/claims` list (pending queue). User token, not application key.

## Spec deviations (with reason)

1. **`series` and `traits` `refs=`** — `catalog_external_ref` has no entity_type for series or trait (`constants.go` entity types stop at engine=8). Unknown refs go to `missing[]`. Not a new table.

2. **Character full fields use `omitempty`** — D30 says unrecorded is JSON `null`. `view=basic` must not emit those keys (default-thin). Unrecorded values on `view=full` are therefore omitted rather than `null`. Same projection trick as work `include=` blocks.

3. **Trait JSON uses `is_sexual`, not `sexual`** — G8: `Screenshot.sexual` is `*string` (`safe|suggestive|explicit`). A bool cannot share the property name. Work tags already use `is_sexual`.

4. **Cover vote is `up` only** — v1 stores one ballot per (user, work). There is no down column. `vote=down` is 422.

5. **Playtime GET empty is 404** — v1 returned 200 + null. v2 has no envelope; a missing row is `NOT_FOUND`.

6. **Playtime delete** — v1 had no DELETE. v2 DELETE removes `catalog_user_playtime` rows for that (user, work). Additive service method, existing table.

7. **Claims list is not keyset-paginated yet** — v1 `ClaimsByActor` pages on `last_event_id`. The v2 list currently returns one page. Cursor mapping is still to do.

8. **Moderation claims list is not site-fenced yet** — `PendingClaims` is called with empty site. Production must pass the token client's catalog site once UserGate/AdminGate is applied to these prefixes. Until then the queue is process-wide.

9. **`blood_type` public tokens are lowercase** (`a|b|ab|o`) per 04-representation. D30's table wrote `A|B|AB|O` as domain letters; the editing engine stores int16. Public JSON follows 04.

## Not mounted yet (Stage 6 remainder)

These have v1 backends; they need a v2 handler reshape, not new tables:

| Spec route | Backend | Blocker |
|---|---|---|
| `POST /v2/me/playtimes` 207 batch | `UserPlaytimeService.Report` per item | 207 Multi-Status + per-item problem objects |
| `POST /v2/me/claims` | `SubmitWork` | Body reshape (`work_id`/`refs`/`content_limit` vs v1 `fields`) |
| `GET/PATCH /v2/me/claims/{id}` | `Act` + work row | `{id}` is catalog work id; GET has no dedicated method |
| `GET/POST /v2/me/proposals*` | `editing.Engine` | Need Engine on `Catalog`; PATCH folds withdraw+amend |
| `POST /v2/me/proposals/{id}/amendments` | `AmendProposal` | Direct |
| `GET /v2/moderation/claims/{id}` | work + pending state | No GetClaim |
| `POST /v2/moderation/claims/{id}/decisions` | `Act` approve/decline | If-Match required (428); decline needs reason |
| `GET /v2/moderation/proposals` | `ListProposalsWithTotal` | Site + review perm |
| `POST /v2/moderation/proposals/{id}/decisions` | `MergeProposal` / `DeclineProposal` | If-Match |
| `POST /v2/moderation/reverts` | `Engine.Revert` | Spec `{revision_id}` vs engine `(type,id,seq)` |
| `GET /v2/moderation/snapshots/{object}/{id}` | `CurrentSnapshot` | Direct |

## Stage 7–10 (not this repo's HTTP surface)

- **7 MCP / SDK** — generated from `v2-openapi.yaml` in kungal-docs / client repos.
- **8 preview portal, HTML problem catalog, design-principles page** — `developer.nextmoe.dev`, not `cmd/catalog`.
- **9 GA / 10 v1 410** — product announcement, consumer migration, Sunset headers. Not code-completeable from infra alone.

## Tests that exist

- Spec-driven: `TestContractHitsEverySpecOperation` hits every registered op; undeclared status fails.
- Gates G2–G16 on the generated OpenAPI document.
- Per-bind unit tests for mapping, unbound 503, auth 401, NSFW, cursor, schema `field_type`.
- **Not yet:** live DB integration per route (needs a track-specific `TEST_DATABASE_DSN`). Handler tests without a catalog DB cannot assert 200 bodies for list/detail.

When a test database is assigned, the next slice is: one 200-path integration test per operation in `v2-openapi.yaml`, plus the Stage 6 POST/PATCH/DELETE table above.
