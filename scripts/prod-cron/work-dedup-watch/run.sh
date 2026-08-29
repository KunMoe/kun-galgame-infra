#!/bin/sh
# Weekly duplicate-work watch. Re-runs the standing detector over the live
# catalog and alerts when it finds a pair that nobody — machine or human — has
# decided yet. READ-ONLY: `work-dedup -mode watch` writes nothing; the seed /
# propose / execute lanes are operator-driven and are not run from cron.
#
# Exit-code contract of the tool, which this wrapper depends on:
#   0  detector ran, no undecided new pairs
#   3  detector ran, new pairs found  -> [DUP] alert, and STILL a success stamp:
#      the run was healthy and the finding is the alert. Suppressing the stamp
#      would make the deadman report a working job as dead every week.
#   *  the detector broke            -> [FAIL] alert, no stamp
#
# crontab (root): 20 4 * * 1 /root/work-dedup-watch/run.sh
#
# Runs from the infra-tools image resolved below; nothing is staged on this
# host. Canonical copy: scripts/prod-cron/work-dedup-watch/run.sh in
# kun-galgame-infra — installing or updating it on the box is a manual scp over
# /root/work-dedup-watch/run.sh, so edit here first and copy it out (two chains
# once sat on a stale hand-edited copy for a whole cycle).
set -eu
BASE=/root/work-dedup-watch
cd "$BASE"
mkdir -p logs state
LOG="logs/run-$(date -u +%F).log"
exec >>"$LOG" 2>&1
exec 9>"$BASE/.lock"; flock -n 9 || { echo "another run holds the lock; exit"; exit 0; }
LOCKED=1

# Two-layer reporting, as in the other maintenance crons: the trap catches a run
# that failed, and state/last-success lets /root/lib/watchdog.sh catch a job that
# stopped running at all — the failure no in-script handler can see. The stamp is
# gated on LOCKED so a run that skipped because a previous one still holds the
# lock never stamps for work it did not do.
on_exit() {
  rc=$?
  if [ -f env.tmp ]; then shred -u env.tmp; fi
  if [ "$rc" -eq 0 ] && [ "${LOCKED:-0}" = 1 ]; then
    date -u '+%F %T' > "$BASE/state/last-success"
  else
    echo "=== FAILED (exit $rc) - sending alert ==="
    /root/lib/alert.sh "[FAIL] work-dedup weekly watch (exit $rc)" "$BASE/$LOG" || echo "alert delivery failed"
  fi
}
trap on_exit EXIT
echo "=== work-dedup watch start $(date -u '+%F %T')Z ==="

PG=kun-visual-novel-infra-vqvqbc-postgres-1
CATC=kun-visual-novel-infra-vqvqbc-catalog-1

IMG_TAG=ghcr.io/kunmoe/infra-tools:latest
docker pull -q "$IMG_TAG" >/dev/null 2>&1 || echo "WARN: image pull failed; using the local copy"
IMG=$(docker image inspect --format '{{index .RepoDigests 0}}' "$IMG_TAG")
echo "image: $IMG"

# Fresh env snapshot from the catalog container; the EXIT trap shreds it.
docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$CATC" > env.tmp
chmod 600 env.tmp

# The DSN is assembled INSIDE the container from that snapshot, so the password
# exists only in that process and never in argv on this host. Parallel query is
# disabled per session: the postgres container runs with the 64MB docker-default
# /dev/shm and the pair detector is a self-join over every live work title.
#
# work_mem / hash_mem_multiplier / jit are forced down because parallel-off
# alone was not enough: on 2026-08-29 the first prod census ran under prod's
# work_mem=64MB x hash_mem_multiplier=2 and the planner kept the whole pair
# query in memory — the backend grew to 6.4GB anon RSS, the kernel OOM-killed
# it and postgres went through crash recovery (a ~1.3s full-platform DB
# outage). Dev never showed this because its work_mem=4MB made the identical
# query spill ~5GB to temp files instead. These GUCs pin the disk-spill plan;
# do not remove them to make the watch faster.
DSNSH='U="${KUN_CATALOG_PG_USER:-$KUN_PG_USER}"; P="${KUN_CATALOG_PG_PASSWORD:-$KUN_PG_PASSWORD}"; CAT="host=127.0.0.1 port=5432 user=$U password=$P dbname=${KUN_CATALOG_PG_DATABASE:-kun_catalog} sslmode=disable options='"'"'-c max_parallel_workers_per_gather=0 -c work_mem=8MB -c hash_mem_multiplier=1 -c jit=off'"'"'"'

rc=0
docker run --rm --network "container:$PG" --env-file "$BASE/env.tmp" "$IMG" \
  sh -c "$DSNSH"'; work-dedup -mode watch -fail-on-new -dsn "$CAT"' || rc=$?
case "$rc" in
  0) echo "no new undecided duplicate pairs" ;;
  3) echo "=== new undecided duplicate pairs - sending alert ==="
     /root/lib/alert.sh "[DUP] new duplicate work pairs" "$BASE/$LOG" || echo "alert delivery failed" ;;
  *) echo "FATAL: work-dedup watch exited $rc"; exit "$rc" ;;
esac

find logs -name 'run-*.log' ! -name "run-$(date -u +%F).log" -exec gzip -qf {} \;
find logs -name 'run-*.log.gz' -mtime +90 -delete
echo "=== work-dedup watch done $(date -u '+%F %T')Z ==="
