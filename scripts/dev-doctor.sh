#!/usr/bin/env bash
# Preflight for `pnpm dev`. Turns the two failures that actually happen on a
# fresh checkout — "migrate exited 1" and "nothing can reach anything" — into a
# sentence naming the cause. Run standalone (`pnpm dev:doctor`) or let
# scripts/dev.sh call it.
#
# Read-only: it starts no service, creates no database, changes no config.
set -uo pipefail
cd "$(dirname "$0")/.."

FAIL=0
ok()   { printf '  \033[32m✓\033[0m %s\n' "$*"; }
warn() { printf '  \033[33m!\033[0m %s\n' "$*"; }
bad()  { printf '  \033[31m✗\033[0m %s\n' "$*"; FAIL=$((FAIL + 1)); }
hint() { printf '      %s\n' "$*"; }

echo "▶ dev-doctor"

# ── 1. the shell you are in ───────────────────────────────────────────────────
# Git Bash / MSYS / Cygwin run on Windows proper. Docker there is Docker Desktop,
# whose containers live in a Linux VM, and this whole stack is host-networked.
case "$(uname -s)" in
  MINGW* | MSYS* | CYGWIN*)
    bad "running under $(uname -s) (Git Bash / MSYS) — this stack needs WSL2"
    hint "Install WSL2, clone the repo INSIDE the distro's filesystem (~/, not"
    hint "/mnt/c — 9p filesystem writes are slow enough to break air), and run"
    hint "pnpm dev from a WSL2 shell. docs/dev-environment.md → Windows."
    echo; echo "✗ dev-doctor: cannot continue in this shell."; exit 1 ;;
  Linux)
    if grep -qiE 'microsoft|wsl' /proc/sys/kernel/osrelease 2>/dev/null; then
      ok "WSL2 (${WSL_DISTRO_NAME:-unknown distro})"
    else
      ok "Linux"
    fi ;;
  Darwin) warn "macOS — host networking is a Linux feature; expect the probe below to fail" ;;
esac

# ── 2. tooling ────────────────────────────────────────────────────────────────
for t in docker jq pnpm; do
  if command -v "$t" >/dev/null; then ok "$t present"; else bad "$t not on PATH"; fi
done
docker compose version >/dev/null 2>&1 || bad "docker compose v2 plugin missing (\`docker compose version\`)"
docker info >/dev/null 2>&1 || { bad "docker daemon not reachable"; hint "start Docker, then re-run"; }
((FAIL)) && { echo; echo "✗ dev-doctor: fix the above first."; exit 1; }

# ── 3. Postgres, as the containers will see it ────────────────────────────────
# Coordinates from the rendered compose config, so a root .env override is
# reflected here exactly as the containers get it. The password is read but
# never printed.
eval "$(docker compose -f docker-compose.dev.yml config --format json \
  | jq -r '.services.migrate.environment
           | "PGH=\(.KUN_PG_HOST)\nPGP=\(.KUN_PG_PORT)\nPGU=\(.KUN_PG_USER)"')"

port_open() { (exec 3<>"/dev/tcp/$1/$2") 2>/dev/null && exec 3>&-; }

PG_UP=0
if port_open "$PGH" "$PGP"; then
  PG_UP=1
  ok "Postgres answering on $PGH:$PGP (user $PGU) from this shell"

  # The decisive Windows/macOS check. Every service here is network_mode: host,
  # so "can a host-networked container reach that same port" is the question —
  # under Docker Desktop the answer is no, because "host" means the Linux VM.
  # redis:8-alpine because the default stack pulls it anyway; nothing extra.
  probe=$(docker run --rm --network host redis:8-alpine \
            sh -c "nc -z -w3 $PGH $PGP && echo REACH || echo NOPE" 2>/dev/null | tail -1)
  if [[ "$probe" == REACH ]]; then
    ok "a host-networked container reaches it too"
  else
    bad "a host-networked container CANNOT reach $PGH:$PGP"
    hint "This is the Docker Desktop shape: the container's 127.0.0.1 is the"
    hint "Linux VM's loopback, not your machine's. Pointing Postgres at"
    hint "host.docker.internal would not help — the services also reach EACH"
    hint "OTHER on 127.0.0.1. Run Postgres inside the same WSL2 distro, or"
    hint "point KUN_PG_HOST at an address the VM can route to (root .env)."
  fi
else
  bad "nothing listening on $PGH:$PGP"
  hint "Start your Postgres server, or set KUN_PG_HOST / KUN_PG_PORT in a root"
  hint ".env (gitignored, next to docker-compose.dev.yml)."
fi

# ── 4. the databases ──────────────────────────────────────────────────────────
if ((!PG_UP)); then
  warn "skipped the database check — the server above has to answer first"
elif command -v psql >/dev/null; then
  if out=$(./scripts/create-dev-databases.sh --check 2>&1); then
    ok "$(echo "$out" | tail -1 | sed 's/^✓ //')"
  elif missing=$(echo "$out" | grep '· missing') && [[ -n "$missing" ]]; then
    bad "databases missing:"
    echo "$missing" | sed 's/^/    /'
    hint "./scripts/create-dev-databases.sh    # creates them, idempotent"
  else
    # Reached the server's TCP port but psql itself refused — a wrong password
    # looks exactly like this, and it is the other half of "migrate exited 1".
    bad "psql could not query the server (port is open, so: credentials or auth)"
    echo "$out" | sed 's/^/    /'
  fi
else
  warn "psql not on PATH — cannot verify the databases exist"
  hint "A migrate job that exits 1 seconds after start is almost always this."
  hint "sudo apt install postgresql-client, then ./scripts/create-dev-databases.sh"
fi

# ── 5. GHCR (platform images; inherit repo access if linked) ──────────────────
if docker manifest inspect ghcr.io/next-moe/infra-migrate:latest >/dev/null 2>&1; then
  ok "GHCR: infra-migrate readable"
else
  bad "cannot read ghcr.io/next-moe/infra-migrate"
  hint "A bare \`gh auth token\` lacks read:packages and fails 'unauthorized':"
  hint "  gh auth refresh -h github.com -s read:packages"
  hint "  gh auth token | docker login ghcr.io -u <your-gh-user> --password-stdin"
fi

echo
((FAIL)) && { echo "✗ dev-doctor: $((FAIL)) area(s) need attention (details above)."; exit 1; }
echo "✓ dev-doctor: ready for pnpm dev."
