# v2 implementation notes (preview)

This page records **binds that differ from `refs/api-v2`** and **Stage 6–8 work that is not mounted yet**. It is not a substitute for the design spec. Code + `docs/catalog/v2-openapi.yaml` are the machine-readable contract.

Date: 2026-08-23. Branch: `v2-stage0`. No push (deploy is automatic).

## Done on this branch

- Protocol, problems, vocabularies, collection contract, repr types, CI gates G1–G16.
- Catalog read: works (list/detail/12 subs), companies+graph, tags, series, engines, releases, characters, credit-names, persons, traits, search, calendar, changes, redirects, stats, schemas/{object}, news.
- Character `view=full` carries D30 attributes (`gender`, `birthday` as `MM-DD`, measurements, `blood_type` as `a|b|ab|o`, `instance_of_id`). `description` / `extra` / `field_provenance` stay out, as D30.
- `/v2/me/playtimes` GET/PUT/DELETE and POST 207 batch; `/v2/me/cover-votes` GET/PUT/DELETE.
- `/v2/me/claims` list/create/get/withdraw; `/v2/me/proposals` list/create/get/patch/amend.
- `/v2/moderation/claims` queue + decisions; `/v2/moderation/proposals` queue + decisions; reverts; snapshots.
- User token on `/v2/me` and `/v2/moderation`, not an application key; `private, no-store`.

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

## Stage 6 write (mounted 2026-08-23)

| Route | Bind |
|---|---|
| `POST /v2/me/playtimes` | 207 list of playtime-or-problem items |
| `POST /v2/me/claims` | `work_id` → `Act(claim)`; else `SubmitWork` (needs `display_name` to mint) |
| `GET/PATCH /v2/me/claims/{id}` | `{id}` is catalog work id. PATCH `{state:withdrawn}` + If-Match |
| `GET/POST /v2/me/proposals` | `editing.Engine` |
| `GET/PATCH /v2/me/proposals/{id}` | PATCH withdraw or amend; If-Match |
| `POST /v2/me/proposals/{id}/amendments` | `AmendProposal` + If-Match |
| `POST /v2/moderation/claims/{id}/decisions` | approve/decline + If-Match |
| `GET /v2/moderation/proposals` | open proposals, site-fenced when the token client has a catalog site |
| `POST /v2/moderation/proposals/{id}/decisions` | merge/decline + If-Match |
| `POST /v2/moderation/reverts` | `revision_id` loads `edit_revision` then `Revert` |
| `GET /v2/moderation/snapshots/{object}/{id}` | `CurrentSnapshot` |

Still missing vs 03/06:

- `GET /v2/moderation/claims/{id}` as its own read (queue list exists)
- Claim POST `refs` as source:external_id (SubmitWork anchors are URL `links`, not v2 refs)
- Per-field `HasPerm` depends on JWT `roles`; a token without catalog edit perms will 403 `PERMISSION_REQUIRED` on propose/review — same as v1, not a new identity system.

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
