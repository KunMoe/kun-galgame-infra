// Chinese overlay for the developer portal's API reference.
//
// The English DocsModel (docs-model.ts) and the LLM Markdown twins stay the
// contract. This overlay is a gettext-style map of those English strings to
// Simplified Chinese, applied only when the HTML pages render. Identifiers
// (nsfw, refs, work_id, ENTITY_MERGED, …) stay in Latin inside the Chinese
// prose; they are never keys of their own.
//
// sync-specs.mjs builds the model, then asserts this catalog covers every
// translatable string on it and contains no leftover keys. A spec edit that
// changes a description is therefore a failed docs-model job until the
// overlay is updated — the same drift shape as the generated model itself.

import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))
export const DOCS_ZH_CATALOG = join(__dirname, '..', 'i18n', 'docs-zh.json')

export const collectDocStrings = (model) => {
  const texts = new Set()
  const add = (value) => {
    if (typeof value === 'string' && value.trim()) texts.add(value.trim())
  }
  const walkNode = (node) => {
    if (!node) return
    add(node.doc)
    node.children?.forEach(walkNode)
    if (node.itemsOf) walkNode(node.itemsOf)
  }
  for (const face of model.faces) {
    for (const group of face.groups) {
      for (const op of group.operations) {
        add(op.summary)
        add(op.description)
        op.params?.forEach((param) => add(param.doc))
        walkNode(op.requestBody)
        op.responses?.forEach((res) => {
          add(res.description)
          walkNode(res.schema)
        })
      }
    }
  }
  return texts
}

export const loadDocZhCatalog = (path = DOCS_ZH_CATALOG) => {
  const raw = JSON.parse(readFileSync(path, 'utf8'))
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
    throw new Error(
      `docs-zh overlay must be a JSON object of english → 中文: ${path}`
    )
  }
  return raw
}

export const assertDocZhCatalog = (model, catalog = loadDocZhCatalog()) => {
  const needed = collectDocStrings(model)
  const missing = []
  const empty = []
  for (const text of needed) {
    const zh = catalog[text]
    if (typeof zh !== 'string') missing.push(text)
    else if (!zh.trim()) empty.push(text)
  }
  const extra = Object.keys(catalog).filter((key) => !needed.has(key))
  const problems = []
  if (missing.length) {
    problems.push(
      `${missing.length} docs-zh overlay entries missing (English source, first 8):\n` +
        missing
          .slice(0, 8)
          .map((text) => `  - ${JSON.stringify(text)}`)
          .join('\n')
    )
  }
  if (empty.length) {
    problems.push(
      `${empty.length} docs-zh overlay entries are empty:\n` +
        empty
          .slice(0, 8)
          .map((text) => `  - ${JSON.stringify(text)}`)
          .join('\n')
    )
  }
  if (extra.length) {
    problems.push(
      `${extra.length} stale docs-zh overlay keys (no longer on the model, first 8):\n` +
        extra
          .slice(0, 8)
          .map((text) => `  - ${JSON.stringify(text)}`)
          .join('\n')
    )
  }
  if (problems.length) {
    throw new Error(
      `docs-zh overlay is out of date vs the public spec.\n` +
        `Update apps/developer/i18n/docs-zh.json, then re-run sync:specs.\n\n` +
        problems.join('\n\n')
    )
  }
}
