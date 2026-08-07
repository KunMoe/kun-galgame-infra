#!/usr/bin/env bash
#
# config.sh — single source of truth for the dev-seed pipeline.
#
# The dev-seed pipeline samples a few hundred entities (with full FK closure)
# out of the LOCAL desensitised snapshot databases (which scripts/refresh-dev-db.sh
# maintains) and publishes tiny per-DB dumps as a GitHub Release, so that
# collaborators can bootstrap a working dev database set without any access to
# the production server or to the full snapshot artifacts.
#
# Desensitisation is inherited: the sampling source is the local copy that the
# refresh pipeline already scrubbed (scripts/dev-snapshot/scrub/*.sql), so no
# scrub step exists here on purpose. Never point SEED_SOURCE at raw prod data.
#
# shellcheck disable=SC2034  # every value here is consumed by a sourcing script.

# Reuse the local Postgres coordinates (LOCAL_PG_*) and dev constants from the
# snapshot pipeline — one truth for both pipelines.
DEV_SEED_CONFIG_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../dev-snapshot/config.sh
source "${DEV_SEED_CONFIG_DIR}/../dev-snapshot/config.sh"

# Databases to seed, in build order. Exporter DBs (whose prune step \copy's
# kept-user id lists into the work dir) MUST run before importer DBs (whose
# prune step reads those lists to keep the matching platform-side rows).
SEED_DBS=(
  kungalgame          # exports kept user / topic / galgame / resource id lists
  kungalgame_patch    # exports kept patch-local user + patch (= wiki gid) ids
  kun_catalog         # imports both sites' gid lists (curated work anchors)
  kun_community       # imports forum content ids; exports kept platform uids
  kun_galgame_infra   # imports all kept-user lists (accounts + site mappings)
  kun_images
  kun_artifacts
)

# Scratch database name suffix. build-seed.sh only ever drops/creates
# databases carrying this suffix — never the snapshot-managed names.
SEED_BUILD_SUFFIX="_seedbuild"

# Rough sampling targets (each prune/<db>.sql consumes what it needs via -v).
SEED_WORKS=300        # catalog works / forum galgame entries / patches
SEED_TOPICS=300       # forum topics
SEED_USERS=400        # per-site sampled users (content authors are always kept)

# Where build artifacts land (outside the repo; safe for a systemd timer).
SEED_OUT_ROOT="${HOME}/.cache/kungal-dev-seed"

# Publishing target: a rolling release on a fixed tag, assets replaced in
# place. Lives on the infra repo itself (PUBLIC, so the seed is world-
# downloadable) — ruled acceptable 2026-08-06: the seed is fully desensitised
# (scrubbed content, dev-only credentials, private-submission tables shipped
# empty), and reusing the repo the collaborators already have beats
# maintaining a separate invite list.
SEED_REPO="KunMoe/kun-galgame-infra"
SEED_TAG="dev-seed"
