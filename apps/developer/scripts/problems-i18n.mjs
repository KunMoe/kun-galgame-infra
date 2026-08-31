// Chinese overlay for the RFC 9457 problem registry.
//
// app/generated/problems.ts is written by cmd/gen-v2-portal from the Go
// registry and stays English: `code`, `title` and `description` are the wire
// contract, and every /problems/{domain}/{kebab} URL is a `type` an agent may
// fetch. This file only adds a Simplified Chinese twin for the HTML pages —
// they render both, Chinese for the reader and the English original underneath.
//
// Keyed by the English source string, gettext-style, the same shape as
// docs-i18n.mjs: rewording an English description orphans its translation and
// fails the assert, where a code-keyed map would silently keep the stale one.
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))
export const PROBLEMS_MODEL = join(__dirname, '..', 'app/generated/problems.ts')
export const PROBLEMS_ZH_CATALOG = join(
  __dirname,
  '..',
  'i18n',
  'problems-zh.json'
)

// The model is a Go-generated TS module, not JSON, and this script must not
// depend on Node's type stripping to read it: slice the array literal out and
// parse that. The generator emits exactly one array, `] as const` terminated.
export const loadProblems = (path = PROBLEMS_MODEL) => {
  const raw = readFileSync(path, 'utf8')
  const start = raw.indexOf('[')
  const end = raw.lastIndexOf(']')
  if (start < 0 || end < start) {
    throw new Error(`no problem array found in ${path}`)
  }
  return JSON.parse(raw.slice(start, end + 1))
}

export const loadProblemZhCatalog = (path = PROBLEMS_ZH_CATALOG) => {
  const raw = JSON.parse(readFileSync(path, 'utf8'))
  if (!raw?.domains || !raw?.strings) {
    throw new Error(`problems-zh overlay needs { domains, strings }: ${path}`)
  }
  return raw
}

export const buildProblemsZh = (
  problems = loadProblems(),
  catalog = loadProblemZhCatalog()
) => {
  const needed = new Set()
  const domains = new Set()
  for (const p of problems) {
    needed.add(p.title)
    needed.add(p.description)
    domains.add(p.domain)
  }

  const missing = [...needed].filter((t) => !catalog.strings[t]?.trim())
  const extra = Object.keys(catalog.strings).filter((k) => !needed.has(k))
  const missingDomains = [...domains].filter((d) => !catalog.domains[d]?.trim())
  const extraDomains = Object.keys(catalog.domains).filter(
    (d) => !domains.has(d)
  )
  const faults = []
  const list = (label, items) =>
    items.length &&
    faults.push(
      `${items.length} ${label}:\n` +
        items
          .slice(0, 8)
          .map((t) => `  - ${JSON.stringify(t)}`)
          .join('\n')
    )
  list('problems-zh entries missing or empty (English source)', missing)
  list('stale problems-zh keys (no longer on the registry)', extra)
  list('problem domains with no Chinese label', missingDomains)
  list('stale problem domain labels', extraDomains)
  if (faults.length) {
    throw new Error(
      `problems-zh overlay is out of date vs the generated registry.\n` +
        `Update apps/developer/i18n/problems-zh.json, then re-run sync:specs.\n\n` +
        faults.join('\n\n')
    )
  }

  return {
    domains: Object.fromEntries(
      [...domains].map((d) => [d, catalog.domains[d]])
    ),
    entries: Object.fromEntries(
      problems.map((p) => [
        p.code,
        {
          title: catalog.strings[p.title],
          description: catalog.strings[p.description]
        }
      ])
    )
  }
}
