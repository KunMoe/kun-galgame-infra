# Prod maintenance crons (kungal-neo)

Canonical copies of every root-owned maintenance cron on the prod box, plus
the shared alert/deadman library. **The deployed copy is `/root/<job>/run.sh`
(and `/root/lib/*`); it must stay byte-identical to this directory.** Deploy a
change by scp'ing the edited file over the deployed path — there is no other
sync mechanism, which is why these live in the repo: review happens here, and
a PR that changes a tool's flags can fix its invocation in the same diff.

## The invariant these scripts exist to keep

**Every Go tool runs from the `infra-tools` image, resolved to a digest at
run start.** No binary is ever staged on the host. The image is built by CI in
lockstep with `apps/api/**`, carries all `cmd/*` binaries on PATH plus tzdata,
and defaults to uid 10001 (`appuser`) — jobs that write root-owned state under
their mount run with `--user 0:0`.

This replaced hand-staged static binaries under `/root/*/bin` on 2026-08-13.
The incident that earned it: the 2026-08-05 bgm-refresh ran a stale
`backfill-work-tags` whose model predated two NOT NULL columns — 120,646
inserts failed, the tool exited 0, and both alert layers stayed silent for 15
days. A staged binary is admin code that does not ship with the application;
the image is the fix, not a preference.

## Jobs

| job | schedule (UTC) | deadman limit |
|---|---|---|
| bgm-refresh | Wed 11:00 | 192h |
| vndb-refresh | Sun 17:30 | 192h |
| reindex-catalog | daily 06:10 | 48h |
| intromt-nightly | daily 13:00 | 48h |
| image-grade-nightly | daily 15:00 | 48h |
| playtime-aggregate | daily 05:20 | 48h |
| refresh-tag-counts | hourly :40 | 6h |
| tag-vocab-backlog | 1st 12:30 | 768h |
| work-dedup-watch | Mon 04:20 | 192h |

Crontab (root) also runs `/root/lib/watchdog.sh` daily at 09:00 — the deadman
that alerts when a job has not *succeeded* within its limit, the failure mode
no in-script trap can see. Registry: `lib/watchdog.conf`.

## Layout on the box

Each job owns `/root/<job>/` with `run.sh`, `logs/`, `state/` (last-success
stamp + job-specific state), and job-local env files where needed
(intromt-nightly keeps `app.env` + `llm.env`; image-grade-nightly reads
`/root/imgsafety/env.img` + `env.cf`, left in place by the full-corpus grading
run). Secrets never live in these
scripts: DB env is snapshotted from the live catalog container per run and
shredded on exit; DSNs are assembled inside the tool container, never on a
host command line.
