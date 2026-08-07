-- =============================================================================
-- dev-seed prune: kungalgame_patch (moyu.moe patch site)
-- =============================================================================
-- Shrinks a full scratch copy (TEMPLATE'd from the desensitised prod snapshot)
-- down to the newest :seed_works patches plus full FK closure, for a tiny
-- collaborator seed dump.
--
-- Invocation (orchestrator):
--   psql -h localhost -p 5432 -U postgres -v ON_ERROR_STOP=1 -q \
--     -d kungalgame_patch_seedbuild \
--     -v export_dir=<dir> -v seed_works=300 -v seed_topics=300 -v seed_users=400 \
--     -f prune/kungalgame_patch.sql
--
-- Variables used: :seed_works, :seed_users, :export_dir.
-- :seed_topics is accepted but intentionally unused — this database has no
-- topic concept (forum topics live in the platform DB).
--
-- Technique: FK constraints stay enforced. Keep-sets are materialised in TEMP
-- tables, then children are deleted before parents; any closure mistake fails
-- loudly as an FK violation. Fully deterministic (all sampling is ORDER BY id).
--
-- Schema notes discovered against the 2026-08 snapshot:
--   * The desensitised "user" table carries NO role / name / email columns,
--     so "admins & moderators" are approximated by the distinct actors of
--     admin_log (18 users in the snapshot).
--   * Soft (non-FK) reference columns: patch.creator_id, doc.author_uid,
--     doc.user_id, claim_event_processed.actor_uid / work_id,
--     patch_resource_revision.actor_id, patch_resource_file_history.actor_id.
--     Orphan counts for these are reported at the end.
--   * Left untouched (small config / vocab / ledger tables): doc, cron_state,
--     site_setting, _migrations, wiki_message_processed, claim_event_processed,
--     wiki_message_read_state, trust_disposition_applied.
-- =============================================================================

BEGIN;

-- -----------------------------------------------------------------------------
-- Section 0: row counts before pruning (for the change report at the end)
-- -----------------------------------------------------------------------------
CREATE TEMP TABLE _rows_before (tbl text PRIMARY KEY, n bigint NOT NULL);

DO $$
DECLARE t text;
BEGIN
  FOR t IN
    SELECT c.relname FROM pg_class c
    JOIN pg_namespace ns ON ns.oid = c.relnamespace
    WHERE ns.nspname = 'public' AND c.relkind = 'r'
  LOOP
    EXECUTE format(
      'INSERT INTO _rows_before SELECT %L, count(*) FROM %I', t, t);
  END LOOP;
END $$;

-- -----------------------------------------------------------------------------
-- Section 1: keep-sets
-- -----------------------------------------------------------------------------

-- Roots: the newest :seed_works patches by id.
CREATE TEMP TABLE keep_patch (id int PRIMARY KEY);
INSERT INTO keep_patch
SELECT id FROM patch ORDER BY id DESC LIMIT :seed_works;

-- All resources attached to kept patches.
CREATE TEMP TABLE keep_resource (id int PRIMARY KEY);
INSERT INTO keep_resource
SELECT r.id FROM patch_resource r JOIN keep_patch kp ON kp.id = r.galgame_id;

-- All comments on kept patches (whole threads: parent chains never cross
-- patches, so filtering by galgame_id keeps every ancestor of a kept comment).
CREATE TEMP TABLE keep_comment (id int PRIMARY KEY);
INSERT INTO keep_comment
SELECT c.id FROM patch_comment c JOIN keep_patch kp ON kp.id = c.galgame_id;

-- Users to keep:
--   (a) authors of kept patches (patch.user_id, hard FK) and their soft
--       creator_id when it still resolves to a real user;
--   (b) uploaders of kept resources;
--   (c) authors of comments on kept patches;
--   (d) admin_log actors (role columns do not exist in this desensitised
--       snapshot; acting-in-admin_log is the closest admin/moderator signal);
--   (e) the newest :seed_users users by id.
CREATE TEMP TABLE keep_user (id int PRIMARY KEY);
INSERT INTO keep_user
SELECT DISTINCT id FROM (
  SELECT p.user_id       AS id FROM patch p JOIN keep_patch kp ON kp.id = p.id
  UNION
  SELECT p.creator_id    AS id FROM patch p JOIN keep_patch kp ON kp.id = p.id
  WHERE p.creator_id IS NOT NULL
    AND EXISTS (SELECT 1 FROM "user" u WHERE u.id = p.creator_id)
  UNION
  SELECT r.user_id       AS id FROM patch_resource r
  JOIN keep_resource kr ON kr.id = r.id
  UNION
  SELECT c.user_id       AS id FROM patch_comment c
  JOIN keep_comment kc ON kc.id = c.id
  UNION
  SELECT DISTINCT user_id AS id FROM admin_log
  UNION
  -- Parenthesised subselect: a bare ORDER BY/LIMIT here would bind to the
  -- whole UNION and silently cap the entire keep set.
  SELECT id FROM
    (SELECT id FROM "user" ORDER BY id DESC LIMIT :seed_users) newest
) s;

-- Chat rooms kept: every member must be a kept user; cap at the newest 200
-- rooms by id to stay small.
CREATE TEMP TABLE keep_room (id int PRIMARY KEY);
INSERT INTO keep_room
SELECT cr.id FROM chat_room cr
WHERE NOT EXISTS (
  SELECT 1 FROM chat_member m
  LEFT JOIN keep_user ku ON ku.id = m.user_id
  WHERE m.chat_room_id = cr.id AND ku.id IS NULL
)
ORDER BY cr.id DESC LIMIT 200;

-- Chat messages kept: in a kept room, from a kept sender (a sender may have
-- left the room, so membership alone does not guarantee the sender is kept).
CREATE TEMP TABLE keep_chat_message (id int PRIMARY KEY);
INSERT INTO keep_chat_message
SELECT cm.id FROM chat_message cm
JOIN keep_room kr ON kr.id = cm.chat_room_id
JOIN keep_user ku ON ku.id = cm.sender_id;

-- Direct messages kept: both endpoints kept, capped at the ~3,000 newest.
CREATE TEMP TABLE keep_user_message (id int PRIMARY KEY);
INSERT INTO keep_user_message
SELECT um.id FROM user_message um
JOIN keep_user s ON s.id = um.sender_id
JOIN keep_user r ON r.id = um.recipient_id
ORDER BY um.id DESC LIMIT 3000;

-- Admin log kept: the newest 500 entries (all actors are kept via rule (d)).
CREATE TEMP TABLE keep_admin_log (id int PRIMARY KEY);
INSERT INTO keep_admin_log
SELECT id FROM admin_log ORDER BY id DESC LIMIT 500;

ANALYZE keep_patch; ANALYZE keep_resource; ANALYZE keep_comment;
ANALYZE keep_user;  ANALYZE keep_room;     ANALYZE keep_chat_message;
ANALYZE keep_user_message; ANALYZE keep_admin_log;

-- -----------------------------------------------------------------------------
-- Section 2: truncate the stray backup table
-- -----------------------------------------------------------------------------
TRUNCATE TABLE patch_resource_update_time_bak_20260606;

-- -----------------------------------------------------------------------------
-- Section 3: deletes, children before parents
-- -----------------------------------------------------------------------------

-- 3.1 chat subtree ------------------------------------------------------------
DELETE FROM chat_message_seen s
WHERE NOT EXISTS (SELECT 1 FROM keep_chat_message k WHERE k.id = s.chat_message_id)
   OR NOT EXISTS (SELECT 1 FROM keep_user ku WHERE ku.id = s.user_id);

DELETE FROM chat_message_reaction x
WHERE NOT EXISTS (SELECT 1 FROM keep_chat_message k WHERE k.id = x.chat_message_id)
   OR NOT EXISTS (SELECT 1 FROM keep_user ku WHERE ku.id = x.user_id);

DELETE FROM chat_message_edit_history h
WHERE NOT EXISTS (SELECT 1 FROM keep_chat_message k WHERE k.id = h.chat_message_id);

-- reply_to_id is a self-FK ON DELETE SET NULL, so cross-references from kept
-- messages to deleted ones null out automatically.
DELETE FROM chat_message m
WHERE NOT EXISTS (SELECT 1 FROM keep_chat_message k WHERE k.id = m.id);

DELETE FROM chat_member m
WHERE NOT EXISTS (SELECT 1 FROM keep_room kr WHERE kr.id = m.chat_room_id);

DELETE FROM chat_room r
WHERE NOT EXISTS (SELECT 1 FROM keep_room kr WHERE kr.id = r.id);

-- 3.2 comment subtree ---------------------------------------------------------
DELETE FROM user_patch_comment_like_relation l
WHERE NOT EXISTS (SELECT 1 FROM keep_comment kc WHERE kc.id = l.comment_id)
   OR NOT EXISTS (SELECT 1 FROM keep_user ku WHERE ku.id = l.user_id);

DELETE FROM patch_comment c
WHERE NOT EXISTS (SELECT 1 FROM keep_comment kc WHERE kc.id = c.id);

-- 3.3 resource subtree --------------------------------------------------------
DELETE FROM user_patch_resource_like_relation l
WHERE NOT EXISTS (SELECT 1 FROM keep_resource kr WHERE kr.id = l.resource_id)
   OR NOT EXISTS (SELECT 1 FROM keep_user ku WHERE ku.id = l.user_id);

DELETE FROM user_patch_resource_favorite_relation f
WHERE NOT EXISTS (SELECT 1 FROM keep_resource kr WHERE kr.id = f.resource_id)
   OR NOT EXISTS (SELECT 1 FROM keep_user ku WHERE ku.id = f.user_id);

DELETE FROM patch_resource_revision v
WHERE NOT EXISTS (SELECT 1 FROM keep_resource kr WHERE kr.id = v.resource_id);

DELETE FROM patch_resource_file_history h
WHERE NOT EXISTS (SELECT 1 FROM keep_resource kr WHERE kr.id = h.resource_id);

DELETE FROM patch_resource r
WHERE NOT EXISTS (SELECT 1 FROM keep_resource kr WHERE kr.id = r.id);

-- 3.4 remaining patch children ------------------------------------------------
DELETE FROM patch_link l
WHERE NOT EXISTS (SELECT 1 FROM keep_patch kp WHERE kp.id = l.galgame_id);

DELETE FROM user_patch_favorite_relation f
WHERE NOT EXISTS (SELECT 1 FROM keep_patch kp WHERE kp.id = f.galgame_id)
   OR NOT EXISTS (SELECT 1 FROM keep_user ku WHERE ku.id = f.user_id);

DELETE FROM user_patch_contribute_relation c
WHERE NOT EXISTS (SELECT 1 FROM keep_patch kp WHERE kp.id = c.galgame_id)
   OR NOT EXISTS (SELECT 1 FROM keep_user ku WHERE ku.id = c.user_id);

-- 3.5 user children -----------------------------------------------------------
DELETE FROM user_message m
WHERE NOT EXISTS (SELECT 1 FROM keep_user_message k WHERE k.id = m.id);

DELETE FROM admin_log a
WHERE NOT EXISTS (SELECT 1 FROM keep_admin_log k WHERE k.id = a.id);

DELETE FROM user_follow_relation f
WHERE NOT EXISTS (SELECT 1 FROM keep_user ku WHERE ku.id = f.follower_id)
   OR NOT EXISTS (SELECT 1 FROM keep_user ku WHERE ku.id = f.following_id);

-- 3.6 parents -----------------------------------------------------------------
-- patch.user_id is ON DELETE RESTRICT: a mistake in keep_user would abort here.
DELETE FROM patch p
WHERE NOT EXISTS (SELECT 1 FROM keep_patch kp WHERE kp.id = p.id);

DELETE FROM "user" u
WHERE NOT EXISTS (SELECT 1 FROM keep_user ku WHERE ku.id = u.id);

-- -----------------------------------------------------------------------------
-- Section 4: acceptance gate — no table may exceed 10,000 rows
-- -----------------------------------------------------------------------------
DO $$
DECLARE t text; c bigint;
BEGIN
  FOR t IN
    SELECT cl.relname FROM pg_class cl
    JOIN pg_namespace ns ON ns.oid = cl.relnamespace
    WHERE ns.nspname = 'public' AND cl.relkind = 'r'
  LOOP
    EXECUTE format('SELECT count(*) FROM %I', t) INTO c;
    IF c > 10000 THEN
      RAISE EXCEPTION 'acceptance gate failed: table % still has % rows (max 10000)', t, c;
    END IF;
  END LOOP;
END $$;

-- -----------------------------------------------------------------------------
-- Section 5: change report (before -> after, changed tables only)
-- -----------------------------------------------------------------------------
CREATE TEMP TABLE _rows_after (tbl text PRIMARY KEY, n bigint NOT NULL);

DO $$
DECLARE t text;
BEGIN
  FOR t IN
    SELECT cl.relname FROM pg_class cl
    JOIN pg_namespace ns ON ns.oid = cl.relnamespace
    WHERE ns.nspname = 'public' AND cl.relkind = 'r'
  LOOP
    EXECUTE format('INSERT INTO _rows_after SELECT %L, count(*) FROM %I', t, t);
  END LOOP;
END $$;

\echo '=== prune report: rows before -> after (changed tables only) ==='
SELECT b.tbl, b.n AS before, a.n AS after
FROM _rows_before b
JOIN _rows_after a USING (tbl)
WHERE b.n <> a.n
ORDER BY b.tbl;

\echo '=== soft-reference orphan counts (no enforced FK; informational) ==='
SELECT 'patch.creator_id -> user' AS soft_ref, count(*) AS orphans
FROM patch p
WHERE p.creator_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = p.creator_id)
UNION ALL
SELECT 'doc.author_uid -> user', count(*)
FROM doc d
WHERE NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = d.author_uid)
UNION ALL
SELECT 'doc.user_id -> user', count(*)
FROM doc d
WHERE d.user_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = d.user_id)
UNION ALL
SELECT 'claim_event_processed.actor_uid -> user', count(*)
FROM claim_event_processed e
WHERE NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = e.actor_uid)
UNION ALL
SELECT 'claim_event_processed.work_id -> patch', count(*)
FROM claim_event_processed e
WHERE NOT EXISTS (SELECT 1 FROM patch p WHERE p.id = e.work_id)
UNION ALL
SELECT 'patch_resource_revision.actor_id -> user', count(*)
FROM patch_resource_revision v
WHERE NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = v.actor_id)
UNION ALL
SELECT 'patch_resource_file_history.actor_id -> user', count(*)
FROM patch_resource_file_history h
WHERE NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = h.actor_id);

COMMIT;

-- -----------------------------------------------------------------------------
-- Section 6: export kept user ids for the platform DB prune
-- -----------------------------------------------------------------------------
-- One id per line, no header. The platform prune script consumes this file.
-- Note: psql's \copy does NOT interpolate variables in its argument (verified
-- on psql 18), so redirect COPY TO STDOUT through \o instead — \o does
-- interpolate its filename argument.
\set filepath :export_dir '/kungalgame_patch_users.csv'
\o :filepath
COPY (SELECT id FROM "user" ORDER BY id) TO STDOUT WITH (FORMAT csv);
\o
\echo '=== exported kept user ids ==='
\echo :filepath

-- Kept patch ids too: moyu is gid-native (patch.id IS the wiki galgame id),
-- so the catalog prune must keep the works these ids anchor to via the
-- `curated` external-ref source, or every moyu card's catalog hydration
-- misses in the seed.
\set filepath :export_dir '/kungalgame_patch_patches.csv'
\o :filepath
COPY (SELECT id FROM patch ORDER BY id) TO STDOUT WITH (FORMAT csv);
\o
\echo '=== exported kept patch ids ==='
\echo :filepath
