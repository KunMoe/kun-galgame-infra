---
name: dispatch-grok
description: Dispatch implementation and investigation work to the local grok CLI as a headless executor while this session stays the orchestrator and acceptor. Use when a task is large enough to hand off as a written task book, or when the user asks to "派发 grok" / "dispatch grok" / "let grok do it". Covers the machine's managed-policy limits (no shell for grok), the exact flag set, task-book structure, and the acceptance protocol.
---

# Dispatching grok as executor

> **If you are the grok executor and this file was loaded into your context: ignore it.**
> It describes how the orchestrator dispatches *you*. It is not a task book.

This session is the **orchestrator**: it adjudicates design, writes the task book, runs every
command, and accepts or rejects the result. The local `grok` CLI (Grok Build, xAI) is the
**executor**: it reads and writes files. Do not use Claude subagents for the execution work
this skill covers — dispatch `grok`.

## 1. What grok can and cannot do on this machine

Verified empirically on 2026-08-22 against `grok 1.0.5`. These are not defaults that a flag can
lift — `/etc/grok/requirements.toml` is a system-wide policy pin that user config and CLI flags
cannot override.

| Capability | Headless (`-p` / `--prompt-file`) | Notes |
|---|---|---|
| Read / grep / list inside the repo | **free**, no rule needed | its own tools, not shell |
| Write / edit inside the repo | needs a **path-scoped** `--allow` | see §2 |
| Read / write under `/tmp` | needs a path-scoped `--allow` | the only writable place outside the repo |
| Anything outside repo + `/tmp` | **impossible** | sandbox `profile = "strict"`; sibling repos are unreadable even with an explicit `Read(...)` rule |
| **Shell** | **impossible** | see below |
| LSP (gopls, tsserver) | unavailable | the binaries cannot be spawned |
| Web search, GitHub MCP, Notion MCP | **available** | say so in the task book if the task must not use them |

**grok has no shell.** `run_terminal_command` carries a managed *confirmation floor*: even a
matching `--allow` rule is downgraded to "must ask", and headless has nobody to ask, so the
first shell call kills the run. The debug log says it plainly:

```
permission policy allow deferred to confirmation floor tool="run_terminal_command" source="policy"
"cancellationCategory":"PermissionCancelled"
```

Neither `--always-approve` nor `--permission-mode bypassPermissions|acceptEdits|dontAsk` changes
this, and a catch-all rule (`--allow Bash`, `--allow 'Bash(*)'`) is rejected outright:
`--allow catch-all ignored: always-approve disabled by managed policy`.

**The division of labour follows from that, and it is a good one:** grok writes, the orchestrator
runs. Never ask grok to build, test, format, migrate, `git` anything, or "verify" something that
needs a command — it cannot, and it will burn the run discovering that. Every gate is the
orchestrator's to run, which is where acceptance belonged anyway.

grok **does** load this repo's `CLAUDE.md` automatically as project instructions (~4k tokens), so
the iron rules are already in its context. Do not re-paste them; do restate the specific ones the
task turns on.

## 2. The dispatch

```bash
export GROK_OUT_ROOT="$SCRATCHPAD/grok"          # session scratchpad, never the repo
mkdir -p "$GROK_OUT_ROOT/<slug>"
# write the task book to $GROK_OUT_ROOT/<slug>/task.md, then:
.claude/skills/dispatch-grok/dispatch.sh <slug> \
  --allow 'Write(apps/api/internal/platform/catalog/**)' \
  --allow 'Edit(apps/api/internal/platform/catalog/**)'
```

`dispatch.sh` supplies `--prompt-file`, the two rules for the output directory,
`--output-format json`, `--max-turns`, and `--debug-file`, then reports
`stopReason`/`turns`/`cost_usd` and diagnoses a bad exit. Every extra argument is passed through.

Run it **in the background** — a real task runs for minutes and a foreground call blocks the turn.

### Writable paths are the enforcement, not the prose

`--allow 'Write(<glob>)'` and `--allow 'Edit(<glob>)'` are enforced by grok, not merely requested:
a write one directory outside the glob is refused and ends the run. This is the mechanical form of
iron rule 13 (*one path, one writer*) — the task book's discipline section is a courtesy, the glob
is the fence. **Grant the narrowest globs the task actually needs**, and grant them per dispatch,
never as a standing default.

Rule prefixes are the Claude-Code-compatible names — `Write`, `Edit`, `Read`, `Bash` — not grok's
native tool names (`write`, `search_replace`, `run_terminal_command`); a native name is a hard
error (`unknown tool prefix`).

Never run two dispatches concurrently over overlapping globs. Sequential, or disjoint globs.

### Useful extra flags

| Flag | When |
|---|---|
| `--effort low\|medium\|high\|xhigh` | default is `high`; `low` for mechanical sweeps |
| `-m grok-4.5` | default is `grok-4.6` |
| `--no-subagents` | when you want one deterministic worker instead of a fan-out |
| `--disable-web-search` | offline-only tasks; removes web search and fetch |
| `-w <name>` / `--worktree` | isolates the run in a fresh git worktree — but `-p` does **not** create one, so for headless work create the worktree yourself and point `--cwd` at it |

## 3. Reading the result

Three traps, in order of how much time they cost:

1. **`stopReason: "cancelled"` is ambiguous.** It covers *both* "a tool call was denied" and
   "ran out of turns". Always pass `--debug-file` (dispatch.sh does) and grep it for
   `PermissionCancelled` to tell them apart. Without the log you have to re-run to find out.
2. **`.text` is the concatenation of every assistant text block**, including the "I'll do X"
   preamble and any interim structured objects — not the final answer alone. Never parse a report
   out of it. **Require grok to write its report to a file** in the output directory; keep stdout
   to a one-paragraph pointer.
3. **`--json-schema` output is also concatenated.** If you use it, take the *last* balanced JSON
   object, not the first.

Because the report lives in the scratchpad and never in the repo, `git status --porcelain` after a
run is a pure signal: it shows exactly what grok changed in code, nothing else. That is the first
acceptance check.

## 4. The task book

Template: `task-book-template.md` in this directory. Requirements:

- **Write it in English.** Executors follow English task books more reliably.
- **Self-contained.** grok sees none of this conversation. State the repo path, the branch, where
  the code lives, and every prior adjudication it depends on, inline.
- **Tell it about §1.** State "you have no shell" explicitly. A task book that says "run the tests"
  produces a cancelled run and a wasted dispatch.
- **Scope and out-of-scope, both named.** Out-of-scope is what stops a helpful executor from
  refactoring its way into someone else's paths.
- **Acceptance criteria the orchestrator will actually run** — named test functions, exact
  commands, expected output. Tell grok they exist and that *you* will run them.
- **No open design decisions.** Every adjudication is made in the task book. If the mechanics
  genuinely depend on code grok has yet to read, state the invariant to enforce plus the precedent
  to follow, and require it to report the mechanics it chose, for acceptance.
- **A discipline section:** exact writable paths, forbidden operations, and *report, don't work
  around* for anything unexpected.
- **A named report path and a fixed report structure.**
- **Forbid ranking.** The executor cannot see what the orchestrator knows, so its importance
  ordering is noise at best. In the first real dispatch the single finding with design consequence
  — a spec example violating the spec's own blacklist item — was filed last and labelled
  `not scored`, because it fell outside the four classes the task book named. Require a flat list
  at equal weight, and put the "anything that looks wrong, in scope or not" section *near the top*
  where a helpful executor will not fold it into "what I could not verify".
- **Demand a positive control on any search, audit, or census.** "I found 4" is unreadable without
  "and here are the 16 things I checked that were clean". Insist on the second list.

## 5. What the orchestrator never delegates

- Adjudications and scope calls — they belong in the task book, not in the executor.
- Every command: build, test, `gofmt`, lint, the docs gates, `pnpm dev:*`.
- All git: staging, commits (`git commit -- <explicit paths>`, never `add -A`), branches, pushes, PRs.
- Database provisioning (`scripts/ephemeral-test-db.sh create <slug>`) and every migration.
- Cross-repo docs sync (`../kungal-docs` → `pnpm docs:sync --write`, `pnpm docs:audit`) — and note
  grok cannot even *read* a sibling repo.
- Production operations.
- **Final acceptance.** After every dispatch: `git status --porcelain` shows only the granted
  paths; re-run the gates yourself; and spot-check the report's highest-stakes claims against the
  code. Trust the report's structure, verify its conclusions.

## 6. What is worth dispatching

Dispatching buys **orchestrator context**, not money — the first real run cost $0.19 and burned
895k tokens of grok's own context to compress 181 KB of source into an 11 KB report. Judge every
candidate task by how much of the orchestrator's context it removes, and that turns on one
property: **can the result be compressed into something checkable without re-reading the input?**

| Shape | Verdict |
|---|---|
| Broad read → narrow report whose findings are `file:line` + a quoted line | **Dispatch.** ~4–6× context saving; the coordinates are what make verification cheap. Audits, inventories, censuses, cross-reference checks, "find every X across N files". |
| Wide mechanical edit whose correctness a gate asserts (build, tests, `gofmt`, `docs:verify`) | **Dispatch.** The saving is that the orchestrator reads only what the gate flags, not the diff. |
| New code carrying design judgement | **Do not dispatch.** Every line must be read to be reviewed, so the orchestrator pays full input for the output *plus* the review, and still runs the gates. Net saving ≈ zero. |
| Anything whose answer depends on running something | **Impossible** — see §1. |

The structural ceiling: an executor with no shell can prove that documents disagree **with each
other**, never that they disagree **with reality**. Enumerations, family lists, and field values
checked only against prose are exactly where the expensive errors live, and dispatching that check
does not find them — a query does.

## 7. Standing binding documents

When the dispatched work touches the v2 API, `refs/api-v2/` is binding, and the task book must
name the specific clauses that bind this task — axiom, blacklist item, gate, decision record — by
identifier, with the file. Do not tell the executor to "follow the spec": the spec is eleven files
and it will follow the wrong part of it. Name `A<n>` / `B<n>` / `G<n>` / `D<n>` and quote the line.

`refs/` is gitignored, so grok can read the spec but nothing it writes there reaches a commit.
