# Task: <one line, imperative, what gets built>

You are the **executor**. The orchestrator wrote this task book; it is self-contained.
You cannot see the orchestrator's conversation. Everything you need is below.

## Context

Repository: `/home/kun/Desktop/code/website/kun-galgame-infra` (your working directory).
Branch: `<branch>` at `<short sha>`.

<Where the relevant code lives — exact paths. What it does today. Why it is changing.
Every prior adjudication this task depends on, stated inline. If the reader would have to
ask "why this way and not the obvious way", answer it here.>

## Your environment (read this, it is not the usual one)

- **You have no shell.** Any terminal command ends this run immediately. Use only your file
  tools: read, grep, list_dir, write, edit.
- You therefore cannot build, test, format, or run anything. **That is expected.** The
  orchestrator runs every gate after you finish. Write code that compiles; do not try to prove it.
- Reads inside the repository are free. Everything outside it is unreachable except the one
  output directory named below. Sibling repositories cannot be read at all.
- The repository `CLAUDE.md` is already in your context. Its iron rules bind you.
- <Delete if not applicable:> Do not use web search or any MCP tool for this task.

## Binding constraints for this task

<Name the specific clauses, by identifier, with file and quoted line. Not "follow the spec".>

- `refs/api-v2/01-axioms.md` **A8**: "<quoted line>"
- `refs/api-v2/07-governance.md` gate **G17**: "<quoted line>"
- <…>

## Scope

1. <numbered, concrete, each independently checkable>
2. …

## Out of scope

- <what a helpful executor would otherwise wander into>
- Renaming, reformatting, or refactoring anything not named in Scope.

## Precedent to follow

<Point at existing code that already does this correctly: file:line. Say what to copy —
the shape, the error handling, the naming — and what not to.>

## Acceptance criteria

The orchestrator will run these. You cannot. They are listed so you know what your code
must satisfy:

- `<exact command>` → `<expected output>`
- Test `<TestName>` in `<file>` must pass.
- `git status --porcelain` must show **only** the writable paths below.

## Report

Write your report to this exact absolute path:

    <GROK_OUT_ROOT>/<slug>/report.md

Structure:

```
# Report: <task>

## 1. What I changed
(file:line per change, one line each, what and why)

## 2. Anything that looks wrong — in scope or not
(report every one, at the same weight, with file:line and the quoted line. Something outside
 this task's scope belongs here, not in section 5, and not with a note that it was out of scope.
 Report it; do not fix it.)

## 3. Mechanics I chose
(any decision the task book left to the code — what you picked and the precedent you followed)

## 4. Deviations from the task book
(if none, write "None.")

## 5. What I could not verify
(everything requiring a command belongs here — be specific about what the orchestrator
 should check, not just "run the tests")
```

Your final stdout message: one short paragraph, the report path plus a one-line status.
Do not paste the report into stdout.

## Discipline

- Writable paths — **exactly** these, nothing else anywhere:
  - `<glob 1>`
  - `<glob 2>`
  - the report path above
- Forbidden: any shell command; any git operation; any database access; starting any service;
  editing files outside the writable paths; touching `KunUI` / `@kungal/ui-*` sources.
- **Report, don't work around.** If something is missing, contradictory, or blocked, stop and
  write it in section 4 or 5. A blocked task reported accurately is a success; a task completed
  by inventing around the block is not.
- **Do not rank, score, or filter your findings.** You cannot see what the orchestrator knows, so
  you cannot tell which finding matters most. Report every one flat, at equal weight. Never demote
  something to "minor", "cosmetic", "out of the requested classes", or "not scored" — that
  judgement is the orchestrator's and yours will be wrong.
- <Delete unless the task is a search, audit, or census:> **Include a positive control.** A count
  of what you did *not* find is worthless unless the search is known to work. List what you
  checked that came back clean, with counts, so the zero can be believed.
