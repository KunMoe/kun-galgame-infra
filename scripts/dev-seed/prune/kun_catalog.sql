-- =============================================================================
-- dev-seed prune: kun_catalog
--
-- Shrinks a full copy of kun_catalog (run against a *_seedbuild scratch DB,
-- never the source) down to the newest :seed_works works plus full FK closure,
-- so the result can be pg_dump'ed as a tiny collaborator seed.
--
-- Invocation (orchestrator):
--   psql -v ON_ERROR_STOP=1 -q -d kun_catalog_seedbuild \
--     -v seed_works=300 -f prune/kun_catalog.sql
-- (seed_topics / seed_users are accepted but unused here. Reads the kept-id
--  CSVs exported by prune/kungalgame.sql and prune/kungalgame_patch.sql from
--  the CWD, which the orchestrator sets to the shared export dir.)
--
-- Technique: FK constraints stay enforced throughout. Keep-sets are built in
-- TEMP tables first, then children are deleted before parents so any ordering
-- mistake fails loudly instead of corrupting the seed.
--
-- Entity-type constants (docs/catalog/01-service-and-contract.md):
--   0=person 1=credit_name 2=org 3=label 4=character 5=work 6=release
--   7=tag 8=engine   (7/8 and 2 are keep-all families in this seed)
--
-- Kept in full (small vocab/config, ride along):
--   catalog_tag(+intro,+source_map), catalog_character_trait(+parent),
--   catalog_series(+intro), catalog_engine, catalog_platform, catalog_medium,
--   catalog_role, catalog_source(+role_map), catalog_relation_type,
--   catalog_survivorship_rule, catalog_org (empty), the empty galgame_* family.
-- =============================================================================

BEGIN;

-- =============================================================================
-- 1. Keep-sets
-- =============================================================================

-- Roots (deterministic rich mix): the top (:seed_works - 50) works by
-- popularity (max metric value across sources, tie-broken by work id) plus
-- the newest 50 works by id so recent claim-era shapes stay represented.
-- Popularity-ranked roots carry far denser character/release/credit closures
-- than a plain newest-N slice.
CREATE TEMP TABLE keep_work (id bigint PRIMARY KEY) ON COMMIT DROP;
INSERT INTO keep_work
SELECT work_id FROM catalog_work_popularity
GROUP BY work_id
ORDER BY max(value) DESC, work_id DESC
LIMIT (:seed_works - 50);
INSERT INTO keep_work
SELECT id FROM (SELECT id FROM catalog_work ORDER BY id DESC LIMIT 50) newest
ON CONFLICT DO NOTHING;

-- Cross-DB consistency: the kungal forum and moyu patch site hydrate their
-- lists by bridging their local galgame ids (both are wiki-gid-native) to
-- catalog works through catalog_external_ref rows on the `curated` source
-- (`galgame_wiki` during the rename window), entity_type 5 = work. An
-- independently sampled keep_work shares almost nothing with the sites'
-- newest-N samples (measured: 1/300 overlap), which makes every seeded list
-- page render empty. So the works anchored to the sites' kept ids are part
-- of the root set. CSV paths are CWD-relative: \copy does not interpolate
-- psql variables, so build-seed.sh runs this file with CWD = the export dir.
CREATE TEMP TABLE site_gid (id bigint PRIMARY KEY) ON COMMIT DROP;
\copy site_gid FROM 'kungalgame_galgames.csv'
CREATE TEMP TABLE site_pid (id bigint) ON COMMIT DROP;
\copy site_pid FROM 'kungalgame_patch_patches.csv'
INSERT INTO site_gid SELECT id FROM site_pid ON CONFLICT DO NOTHING;

INSERT INTO keep_work
SELECT DISTINCT r.entity_id
FROM catalog_external_ref r
JOIN catalog_source s ON s.id = r.source_id
  AND s.key IN ('curated', 'galgame_wiki')
JOIN site_gid g ON r.external_id = g.id::text
WHERE r.entity_type = 5
ON CONFLICT DO NOTHING;

-- Releases of kept works.
CREATE TEMP TABLE keep_release (id bigint PRIMARY KEY) ON COMMIT DROP;
INSERT INTO keep_release
SELECT id FROM catalog_release WHERE work_id IN (SELECT id FROM keep_work);

-- Characters: linked to kept works (work_character) or referenced by credits
-- of kept works, plus the transitive instance_of ancestor closure (self-FK).
CREATE TEMP TABLE keep_character (id bigint PRIMARY KEY) ON COMMIT DROP;
INSERT INTO keep_character
SELECT DISTINCT character_id FROM catalog_work_character
WHERE work_id IN (SELECT id FROM keep_work)
UNION
SELECT DISTINCT character_id FROM catalog_credit
WHERE work_id IN (SELECT id FROM keep_work) AND character_id IS NOT NULL;

WITH RECURSIVE anc AS (
    SELECT c.instance_of AS id
    FROM catalog_character c JOIN keep_character k ON k.id = c.id
    WHERE c.instance_of IS NOT NULL
    UNION
    SELECT c.instance_of
    FROM catalog_character c JOIN anc ON c.id = anc.id
    WHERE c.instance_of IS NOT NULL
)
INSERT INTO keep_character SELECT id FROM anc ON CONFLICT DO NOTHING;

-- Credit names + persons. These two tables are mutually referencing
-- (credit_name.person_id -> person, person.primary_credit_name_id ->
-- credit_name), so close the pair to a fixpoint.
CREATE TEMP TABLE keep_credit_name (id bigint PRIMARY KEY) ON COMMIT DROP;
INSERT INTO keep_credit_name
SELECT DISTINCT credit_name_id FROM catalog_credit
WHERE work_id IN (SELECT id FROM keep_work);

CREATE TEMP TABLE keep_person (id bigint PRIMARY KEY) ON COMMIT DROP;

DO $$
DECLARE
    n_person integer;
    n_name   integer;
BEGIN
    LOOP
        INSERT INTO keep_person
        SELECT DISTINCT cn.person_id
        FROM catalog_credit_name cn JOIN keep_credit_name k ON k.id = cn.id
        WHERE cn.person_id IS NOT NULL
        ON CONFLICT DO NOTHING;
        GET DIAGNOSTICS n_person = ROW_COUNT;

        INSERT INTO keep_credit_name
        SELECT p.primary_credit_name_id
        FROM catalog_person p JOIN keep_person kp ON kp.id = p.id
        WHERE p.primary_credit_name_id IS NOT NULL
        ON CONFLICT DO NOTHING;
        GET DIAGNOSTICS n_name = ROW_COUNT;

        EXIT WHEN n_person = 0 AND n_name = 0;
    END LOOP;
END $$;

-- Labels: referenced by work_label links of kept works or by kept credits.
-- (catalog_label vocab is ~37k -> pruned to referenced rows.)
CREATE TEMP TABLE keep_label (id bigint PRIMARY KEY) ON COMMIT DROP;
INSERT INTO keep_label
SELECT DISTINCT label_id FROM catalog_work_label
WHERE work_id IN (SELECT id FROM keep_work)
UNION
SELECT DISTINCT label_id FROM catalog_credit
WHERE work_id IN (SELECT id FROM keep_work) AND label_id IS NOT NULL;

-- Unified (entity_type, id) map of every kept entity, for the polymorphic
-- tables (external_ref / revision / redirect / merge / match).
CREATE TEMP TABLE keep_entity (entity_type smallint, entity_id bigint,
                               PRIMARY KEY (entity_type, entity_id)) ON COMMIT DROP;
INSERT INTO keep_entity
SELECT 0, id FROM keep_person
UNION ALL SELECT 1, id FROM keep_credit_name
UNION ALL SELECT 3, id FROM keep_label
UNION ALL SELECT 4, id FROM keep_character
UNION ALL SELECT 5, id FROM keep_work
UNION ALL SELECT 6, id FROM keep_release
-- keep-all vocab families: org(2, table is empty), tag(7), engine(8)
UNION ALL SELECT 7, id FROM catalog_tag
UNION ALL SELECT 8, id FROM catalog_engine;

-- Temporary helper indexes: three FK columns have no index in the schema, so
-- the per-row RI checks fired by the parent DELETEs below would seq-scan the
-- (bloated) child tables and take hours. Constraints stay fully enforced;
-- these plain indexes are dropped again before COMMIT so the dumped seed
-- schema is byte-identical to the source schema.
CREATE INDEX seed_tmp_credit_release ON catalog_credit (release_id);
CREATE INDEX seed_tmp_credit_label   ON catalog_credit (label_id);
CREATE INDEX seed_tmp_person_primary ON catalog_person (primary_credit_name_id);

-- =============================================================================
-- 2. Upstream source-mirror / staging schemas -> empty
-- =============================================================================
-- The source-mirror families (subject*, tags_vn, chars_traits, extlinks,
-- releases_extlinks, person_character, ...) live in dedicated schemas:
-- src_bangumi, src_llm, src_vndb, src_wiki (~3.2 GB total). Verified: not a
-- single FK constraint references into or out of any src_* table, so all of
-- them are emptied. The loop is dynamic so a snapshot that grows a new mirror
-- table still prunes clean; no CASCADE, so if a future FK ever points at a
-- src_* table this fails loudly instead of silently emptying a kept table.
DO $$
DECLARE
    t record;
BEGIN
    FOR t IN SELECT schemaname, tablename FROM pg_tables
             WHERE schemaname LIKE 'src\_%' ESCAPE '\'
             ORDER BY schemaname, tablename LOOP
        EXECUTE format('TRUNCATE TABLE %I.%I', t.schemaname, t.tablename);
    END LOOP;
END $$;

-- The retired galgame_* family (public schema) is already 0 rows everywhere.
-- The only true scratch table in public is tmp_pairs (getchu_id/work_id,
-- no constraints).
TRUNCATE TABLE tmp_pairs;

-- =============================================================================
-- 3. Work family (children first, then catalog_work last in section 8)
-- =============================================================================

-- cover_vote -> work_cover (FK ON DELETE CASCADE, but delete explicitly).
DELETE FROM catalog_cover_vote v
USING catalog_work_cover c
WHERE v.cover_id = c.id AND c.work_id NOT IN (SELECT id FROM keep_work);
DELETE FROM catalog_work_cover      WHERE work_id NOT IN (SELECT id FROM keep_work);

DELETE FROM catalog_work_title      WHERE work_id NOT IN (SELECT id FROM keep_work);
DELETE FROM catalog_work_intro      WHERE work_id NOT IN (SELECT id FROM keep_work);
DELETE FROM catalog_work_tag        WHERE work_id NOT IN (SELECT id FROM keep_work);
DELETE FROM catalog_work_label      WHERE work_id NOT IN (SELECT id FROM keep_work);
DELETE FROM catalog_work_platform   WHERE work_id NOT IN (SELECT id FROM keep_work);
DELETE FROM catalog_work_engine     WHERE work_id NOT IN (SELECT id FROM keep_work);
DELETE FROM catalog_work_playtime   WHERE work_id NOT IN (SELECT id FROM keep_work);
DELETE FROM catalog_work_popularity WHERE work_id NOT IN (SELECT id FROM keep_work);
DELETE FROM catalog_work_rating     WHERE work_id NOT IN (SELECT id FROM keep_work);
DELETE FROM catalog_work_screenshot WHERE work_id NOT IN (SELECT id FROM keep_work);
DELETE FROM catalog_series_member   WHERE work_id NOT IN (SELECT id FROM keep_work);

-- Work<->work relations survive only when both endpoints are kept.
DELETE FROM catalog_work_relation
WHERE a_work_id NOT IN (SELECT id FROM keep_work)
   OR b_work_id NOT IN (SELECT id FROM keep_work);

-- Claim events (soft work_id, no FK) pruned to kept works.
DELETE FROM catalog_claim_event WHERE work_id NOT IN (SELECT id FROM keep_work);

-- =============================================================================
-- 4. Credits, then releases
-- =============================================================================
-- Credits must go before releases / characters / credit_names / labels: a
-- credit references all of them. Kept credits (work kept) only reference kept
-- rows by construction of the keep-sets above.
DELETE FROM catalog_credit  WHERE work_id NOT IN (SELECT id FROM keep_work);
DELETE FROM catalog_release WHERE id NOT IN (SELECT id FROM keep_release);

-- =============================================================================
-- 5. Character family
-- =============================================================================
DELETE FROM catalog_work_character WHERE work_id NOT IN (SELECT id FROM keep_work);
DELETE FROM catalog_character_alias
    WHERE character_id NOT IN (SELECT id FROM keep_character);
DELETE FROM catalog_character_intro
    WHERE character_id NOT IN (SELECT id FROM keep_character);
-- Trait links pruned to kept characters; the trait vocabulary itself
-- (catalog_character_trait + _parent, ~3.3k rows) is kept in full.
DELETE FROM catalog_character_trait_link
    WHERE character_id NOT IN (SELECT id FROM keep_character);
-- Self-FK instance_of: keep_character is ancestor-closed, and NO ACTION FKs
-- are checked at end of statement, so one DELETE is safe.
DELETE FROM catalog_character WHERE id NOT IN (SELECT id FROM keep_character);

-- =============================================================================
-- 6. Credit names <-> persons (circular FK pair)
-- =============================================================================
DELETE FROM catalog_name_alias
    WHERE credit_name_id NOT IN (SELECT id FROM keep_credit_name);
DELETE FROM catalog_person_intro
    WHERE person_id NOT IN (SELECT id FROM keep_person);

-- Break the person -> credit_name half of the cycle on rows that are about to
-- be deleted anyway, so the two DELETEs below cannot trip either FK.
UPDATE catalog_person SET primary_credit_name_id = NULL
WHERE id NOT IN (SELECT id FROM keep_person)
  AND primary_credit_name_id IS NOT NULL;

DELETE FROM catalog_credit_name WHERE id NOT IN (SELECT id FROM keep_credit_name);
DELETE FROM catalog_person      WHERE id NOT IN (SELECT id FROM keep_person);

-- =============================================================================
-- 7. Label family (org table is empty; nothing to do there)
-- =============================================================================
DELETE FROM catalog_label_alias WHERE label_id NOT IN (SELECT id FROM keep_label);
DELETE FROM catalog_label_intro WHERE label_id NOT IN (SELECT id FROM keep_label);
DELETE FROM catalog_label       WHERE id       NOT IN (SELECT id FROM keep_label);

-- =============================================================================
-- 8. Works (parents of everything above)
-- =============================================================================
DELETE FROM catalog_work WHERE id NOT IN (SELECT id FROM keep_work);

-- Helper indexes served their purpose; drop them so the seed schema matches
-- the source schema exactly.
DROP INDEX seed_tmp_credit_release;
DROP INDEX seed_tmp_credit_label;
DROP INDEX seed_tmp_person_primary;

-- =============================================================================
-- 9. Polymorphic (entity_type, entity_id) tables
-- =============================================================================

-- External refs of kept entities only.
DELETE FROM catalog_external_ref r
WHERE NOT EXISTS (SELECT 1 FROM keep_entity k
                  WHERE k.entity_type = r.entity_type AND k.entity_id = r.entity_id);

-- Revisions of kept entities, then capped at the ~2000 newest by id.
DELETE FROM catalog_revision r
WHERE NOT EXISTS (SELECT 1 FROM keep_entity k
                  WHERE k.entity_type = r.entity_type AND k.entity_id = r.entity_id);
DELETE FROM catalog_revision
WHERE id NOT IN (SELECT id FROM catalog_revision ORDER BY id DESC LIMIT 2000);

-- Redirects survive only when they land on a kept entity (old_id is the
-- merged-away id and no longer exists by design).
DELETE FROM catalog_redirect r
WHERE NOT EXISTS (SELECT 1 FROM keep_entity k
                  WHERE k.entity_type = r.entity_type AND k.entity_id = r.current_id);

-- Merge proposals / match candidates: both endpoints must be kept.
DELETE FROM catalog_merge_proposal p
WHERE NOT EXISTS (SELECT 1 FROM keep_entity k
                  WHERE k.entity_type = p.entity_type AND k.entity_id = p.source_entity_id)
   OR NOT EXISTS (SELECT 1 FROM keep_entity k
                  WHERE k.entity_type = p.entity_type AND k.entity_id = p.target_entity_id);
DELETE FROM catalog_match_candidate c
WHERE NOT EXISTS (SELECT 1 FROM keep_entity k
                  WHERE k.entity_type = c.entity_type AND k.entity_id = c.a_id)
   OR NOT EXISTS (SELECT 1 FROM keep_entity k
                  WHERE k.entity_type = c.entity_type AND k.entity_id = c.b_id);
DELETE FROM catalog_match_rejection r
WHERE NOT EXISTS (SELECT 1 FROM keep_entity k
                  WHERE k.entity_type = r.entity_type AND k.entity_id = r.entity_id);

-- =============================================================================
-- 10. Edit engine (soft entity refs; only family 'catalog'/'catalog.work'
--     exists in the data today)
-- =============================================================================
DELETE FROM edit_revision e
WHERE NOT (e.entity_family = 'catalog' AND e.entity_type = 'catalog.work'
           AND e.entity_id IN (SELECT id FROM keep_work));
DELETE FROM edit_proposal p
WHERE NOT (p.entity_family = 'catalog' AND p.entity_type = 'catalog.work'
           AND p.entity_id IN (SELECT id FROM keep_work));
DELETE FROM edit_proposal_amendment a
WHERE a.proposal_id NOT IN (SELECT id FROM edit_proposal);

COMMIT;

-- =============================================================================
-- 11. Acceptance guard: no table may exceed 50,000 rows.
-- =============================================================================
DO $$
DECLARE
    t   record;
    cnt bigint;
BEGIN
    FOR t IN SELECT schemaname, tablename FROM pg_tables
             WHERE schemaname NOT IN ('pg_catalog', 'information_schema') LOOP
        EXECUTE format('SELECT count(*) FROM %I.%I', t.schemaname, t.tablename)
            INTO cnt;
        IF cnt > 50000 THEN
            RAISE EXCEPTION 'prune failed: table %.% still has % rows (> 50000)',
                t.schemaname, t.tablename, cnt;
        END IF;
    END LOOP;
END $$;

ANALYZE;
