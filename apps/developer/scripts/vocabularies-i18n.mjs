// Chinese overlay for the /v2/vocabularies registry.
//
// app/generated/vocabularies.ts is written by cmd/gen-v2-portal and stays
// English: the `value` tokens are wire values and the display_name /
// description strings are what `GET /v2/vocabularies` itself returns. This file
// only adds a Simplified Chinese twin for the HTML pages.
//
// Keyed by the English source string, gettext-style, the same shape as
// docs-i18n.mjs and problems-i18n.mjs. A key may be qualified with the
// vocabulary name — `roster_role/Main` — and the qualified form wins. That is
// gettext's msgctxt and it is not optional here: `Main` means 主要 on
// roster_role and 本篇 on member_kind, and `Publisher` means 发行 on
// attribution_role and 出版社 on company_kind, so a single global entry would
// ship one of each pair wrong with every gate green.
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))
export const VOCAB_MODEL = join(
  __dirname,
  '..',
  'app/generated/vocabularies.ts'
)
export const VOCAB_ZH_CATALOG = join(
  __dirname,
  '..',
  'i18n',
  'vocabularies-zh.json'
)

// Same reason as problems-i18n.mjs: the model is a Go-generated TS module, and
// reading it must not depend on Node's type stripping. Slice the one array
// literal out and parse that.
export const loadVocabularies = (path = VOCAB_MODEL) => {
  const raw = readFileSync(path, 'utf8')
  const start = raw.indexOf('[')
  const end = raw.lastIndexOf(']')
  if (start < 0 || end < start) {
    throw new Error(`no vocabulary array found in ${path}`)
  }
  return JSON.parse(raw.slice(start, end + 1))
}

export const loadVocabZhCatalog = (path = VOCAB_ZH_CATALOG) => {
  const raw = JSON.parse(readFileSync(path, 'utf8'))
  if (!raw?.vocabularies || !raw?.strings) {
    throw new Error(
      `vocabularies-zh overlay needs { vocabularies, strings }: ${path}`
    )
  }
  return raw
}

export const buildVocabulariesZh = (
  vocabularies = loadVocabularies(),
  catalog = loadVocabZhCatalog()
) => {
  const pick = (vocab, source) =>
    catalog.strings[`${vocab}/${source}`] ?? catalog.strings[source]

  const consumed = new Set()
  const missing = []
  for (const vocab of vocabularies) {
    for (const value of vocab.values) {
      for (const source of [value.display_name, value.description]) {
        if (!source) continue
        const qualified = `${vocab.name}/${source}`
        consumed.add(qualified)
        consumed.add(source)
        if (!pick(vocab.name, source)?.trim()) missing.push(qualified)
      }
    }
  }

  const names = new Set(vocabularies.map((v) => v.name))
  const extra = Object.keys(catalog.strings).filter((k) => !consumed.has(k))
  const missingMeta = [...names].filter(
    (n) => !catalog.vocabularies[n]?.label?.trim()
  )
  const extraMeta = Object.keys(catalog.vocabularies).filter(
    (n) => !names.has(n)
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
  list('vocabularies-zh entries missing or empty (vocab/English)', missing)
  list('stale vocabularies-zh keys (no longer on the registry)', extra)
  list('vocabularies with no Chinese label', missingMeta)
  list('stale vocabulary labels', extraMeta)
  if (faults.length) {
    throw new Error(
      `vocabularies-zh overlay is out of date vs the generated registry.\n` +
        `Update apps/developer/i18n/vocabularies-zh.json, then re-run sync:specs.\n\n` +
        faults.join('\n\n')
    )
  }

  return {
    meta: Object.fromEntries(
      vocabularies.map((v) => [v.name, catalog.vocabularies[v.name]])
    ),
    values: Object.fromEntries(
      vocabularies.map((v) => [
        v.name,
        Object.fromEntries(
          v.values.map((value) => [
            value.value,
            {
              displayName: pick(v.name, value.display_name),
              description: pick(v.name, value.description)
            }
          ])
        )
      ])
    )
  }
}
