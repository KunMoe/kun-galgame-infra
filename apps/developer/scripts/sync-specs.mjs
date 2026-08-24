#!/usr/bin/env node
/**
 * Builds the typed documentation model the self-built API reference (/docs/**)
 * renders. Reads the Tier-A specs from the repo's docs/, parses them,
 * dereferences $ref pointers inline into render-friendly schema trees, derives
 * parameter tables / request bodies / responses, generates a ready-to-run curl
 * example per operation, and writes `app/generated/docs-model.ts`.
 *
 * It then hands the same model to scripts/gen-llms.mjs, which writes the
 * LLM-facing half into public/: llms.txt, llms-full.txt, and one Markdown twin
 * per docs route. One entry point on purpose — CI runs this file and nothing
 * else, so a spec change cannot refresh the reference pages while leaving the
 * Markdown twins describing the old contract.
 *
 * The generated files are committed (same pattern as app/assets/kun-icons.ts):
 * derived build artifacts, never hand-edited. Re-run after the specs change:
 *   pnpm --filter developer sync:specs
 *
 * The galgame face was dropped at wave 146 (2026-07-30): its /v1/galgame
 * projection was delisted and its spec deleted.
 *
 * Three spec files, four faces. `public-openapi.yaml` carries catalog (API-key,
 * read-only) and playtime (user-token; no playtime scope required). Modelling
 * those as one face published five playtime operations as `catalog:read` with
 * an `nm_live_` key in the curl sample — a credential that face rejects.
 * `openapi.yaml` is the user-edit subset only: the third face, `edit`, claims
 * `/api/v1/user/catalog/edit` and names every op under that prefix as include
 * or exclude. The rest of that spec (S2S, claims, covers, submit) is a
 * first-party surface and is not portal-documented. `news/public-openapi.yaml`
 * is the fourth face, an API-key face whose scope is grant-only.
 *
 * Operation GROUPING is derived here, not in the specs: the OpenAPI tags put
 * every operation of a face in one bucket (`catalog-public`, `playtime`, …),
 * which is no navigation at all for a 25-operation face. The spec YAML is a
 * frozen contract and must not be edited to carry portal IA.
 */
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { parse as parseYaml } from 'yaml'
import {
  API_HOST,
  CATALOG_SPEC,
  EDIT_EXCLUDED_OPERATION_IDS,
  EXPECTED_OPERATION_COUNTS,
  FACE_GROUPS,
  FACES,
  NEWS_SPEC,
  NO_AUTH,
  OPERATION_AUTH_OVERRIDES,
  PUBLIC_SPEC,
  USER_TOKEN_AUTH,
  V2_SPEC
} from './faces.mjs'
import { writeLlmArtifacts } from './gen-llms.mjs'

const __dirname = dirname(fileURLToPath(import.meta.url))
const OUTPUT = join(__dirname, '..', 'app/generated/docs-model.ts')
const PUBLIC_DIR = join(__dirname, '..', 'public')

const refName = (ref) => ref.split('/').pop()

const splitType = (rawType) => {
  const types = Array.isArray(rawType) ? rawType : rawType ? [rawType] : []
  return {
    primary: types.find((t) => t !== 'null'),
    nullable: types.includes('null')
  }
}

const buildNode = (schema, { name, required, schemas, seen }) => {
  if (schema.$ref) {
    const rn = refName(schema.$ref)
    if (seen.has(rn)) {
      return { ...(name !== undefined && { name }), type: rn, ...(required && { required }) }
    }
    const resolved = schemas[rn]
    if (!resolved) throw new Error(`unresolved $ref: ${schema.$ref}`)
    const next = new Set(seen)
    next.add(rn)
    return buildNode(resolved, { name, required, schemas, seen: next })
  }

  const { primary, nullable } = splitType(schema.type)
  const node = {}
  if (name !== undefined) node.name = name
  if (required) node.required = true
  if (nullable) node.nullable = true
  if (schema.description) node.doc = schema.description
  if (schema.format) node.format = schema.format
  if (schema.enum) node.enum = schema.enum

  if (primary === 'object' && schema.properties) {
    node.type = 'object'
    const req = new Set(schema.required || [])
    node.children = Object.keys(schema.properties)
      .filter((k) => k !== '$schema')
      .map((k) =>
        buildNode(schema.properties[k], {
          name: k,
          required: req.has(k),
          schemas,
          seen
        })
      )
  } else if (
    primary === 'object' &&
    schema.additionalProperties &&
    typeof schema.additionalProperties === 'object'
  ) {
    node.type = 'map'
    node.itemsOf = buildNode(schema.additionalProperties, { schemas, seen })
  } else if (primary === 'array') {
    node.type = 'array'
    node.itemsOf = schema.items
      ? buildNode(schema.items, { schemas, seen })
      : { type: 'any' }
  } else {
    node.type = primary || 'object'
  }
  return node
}

const sampleValue = (schema, { schemas, seen }) => {
  if (schema.$ref) {
    const rn = refName(schema.$ref)
    if (seen.has(rn)) return null
    const next = new Set(seen)
    next.add(rn)
    return sampleValue(schemas[rn], { schemas, seen: next })
  }
  if (schema.enum) return schema.enum[0]
  const { primary } = splitType(schema.type)
  switch (primary) {
    case 'object': {
      if (schema.properties) {
        const req = new Set(schema.required || [])
        const obj = {}
        for (const k of Object.keys(schema.properties)) {
          if (k === '$schema' || !req.has(k)) continue
          obj[k] = sampleValue(schema.properties[k], { schemas, seen })
        }
        return obj
      }
      if (schema.additionalProperties && typeof schema.additionalProperties === 'object') {
        return { key: sampleValue(schema.additionalProperties, { schemas, seen }) }
      }
      return {}
    }
    case 'array':
      return schema.items ? [sampleValue(schema.items, { schemas, seen })] : []
    case 'integer':
    case 'number':
      return 0
    case 'boolean':
      return true
    default:
      return 'string'
  }
}

const samplePathValue = (param) => {
  if (param.enum?.length) return String(param.enum[0])
  return param.type === 'integer' || param.type === 'number' ? '1' : 'value'
}

const buildCurl = (method, path, params, bodyExample, authHeader) => {
  let url = API_HOST + path
  for (const p of params) {
    if (p.in === 'path') url = url.replace(`{${p.name}}`, samplePathValue(p))
  }
  const verb = method.toUpperCase()
  const lines = [verb === 'GET' ? `curl "${url}"` : `curl -X ${verb} "${url}"`]
  if (authHeader) lines.push(`  -H "${authHeader}"`)
  if (bodyExample !== undefined) {
    lines.push('  -H "Content-Type: application/json"')
    lines.push(`  -d '${JSON.stringify(bodyExample)}'`)
  }
  return lines.join(' \\\n')
}

const buildParams = (rawParams = []) => {
  const mapped = rawParams.map((p) => {
    const s = p.schema || {}
    const { primary } = splitType(s.type)
    const param = {
      name: p.name,
      in: p.in,
      required: !!p.required,
      type: primary || 'string'
    }
    if (s.format) param.format = s.format
    if (p.description || s.description) param.doc = p.description || s.description
    if (s.enum) param.enum = s.enum
    return param
  })
  return mapped.sort((a, b) => {
    if (a.in === b.in) return 0
    return a.in === 'path' ? -1 : 1
  })
}

const jsonContent = (content) =>
  content?.['application/json'] || content?.['application/problem+json']

const authForPath = (faceDef, path, method) => {
  if (faceDef.key !== 'v2') return faceDef.auth
  if (path.startsWith('/v2/me/') || path.startsWith('/v2/moderation/')) return USER_TOKEN_AUTH
  if (
    path.startsWith('/v2/problems') ||
    path.startsWith('/v2/vocabularies') ||
    path.startsWith('/v2/news') ||
    path === '/v2/catalog/stats' ||
    path.startsWith('/v2/catalog/schemas/')
  ) {
    return NO_AUTH
  }
  return faceDef.auth
}

const buildOperation = (method, path, op, { schemas, scope, auth }) => {
  const params = buildParams(op.parameters)

  let requestBody
  let bodyExample
  const bodySchema = jsonContent(op.requestBody?.content)?.schema
  if (bodySchema) {
    requestBody = buildNode(bodySchema, { schemas, seen: new Set() })
    bodyExample = sampleValue(bodySchema, { schemas, seen: new Set() })
  }

  const responses = Object.entries(op.responses || {}).map(([status, res]) => {
    const schema = jsonContent(res.content)?.schema
    return {
      status,
      description: res.description || '',
      ...(schema && { schema: buildNode(schema, { schemas, seen: new Set() }) })
    }
  })

  const override = OPERATION_AUTH_OVERRIDES[op.operationId]
  const effective = override || auth

  return {
    id: op.operationId,
    method,
    path,
    summary: op.summary || '',
    ...(op.description && { description: op.description }),
    scope: override ? '' : scope,
    ...(override && { auth: override }),
    params,
    ...(requestBody && { requestBody }),
    responses,
    curl: buildCurl(method, path, params, bodyExample, effective.curl)
  }
}

const METHODS = ['get', 'post', 'put', 'patch', 'delete']

const opKey = (method, path) => `${method.toUpperCase()} ${path}`

const autoGroupDefs = (faceDef, spec) => {
  const defs = faceDef.autoGroups.map((g) => ({ key: g.key, label: g.label, ops: [] }))
  for (const [path, item] of Object.entries(spec.paths || {})) {
    if (!path.startsWith(faceDef.prefix)) continue
    for (const method of METHODS) {
      if (!item[method]) continue
      const group = defs.find((g) => {
        const src = faceDef.autoGroups.find((a) => a.key === g.key)
        return src.match.test(path)
      })
      if (!group) {
        throw new Error(
          `docs-model grouping guard: ${opKey(method, path)} matches no autoGroup of face ${faceDef.key}`
        )
      }
      group.ops.push(opKey(method, path))
    }
  }
  return defs
}

const buildFace = (faceDef, specs) => {
  const spec = specs.get(faceDef.file)
  const schemas = spec.components?.schemas || {}

  const groupDefs = faceDef.autoGroups ? autoGroupDefs(faceDef, spec) : FACE_GROUPS[faceDef.key]
  if (!groupDefs) {
    throw new Error(`docs-model grouping guard: face ${faceDef.key} has no FACE_GROUPS entry`)
  }
  const buckets = new Map(groupDefs.map((g) => [g.key, []]))
  const placed = new Set()

  for (const [path, item] of Object.entries(spec.paths || {})) {
    if (!path.startsWith(faceDef.prefix)) continue
    for (const method of METHODS) {
      const op = item[method]
      if (!op) continue
      if (faceDef.include && !faceDef.include.includes(op.operationId)) continue
      const key = opKey(method, path)
      const group = groupDefs.find((g) => g.ops.includes(key))
      if (!group) {
        throw new Error(
          `docs-model grouping guard: ${key} (${op.operationId}) belongs to no group of face ${faceDef.key} — add it to FACE_GROUPS`
        )
      }
      placed.add(key)
      const auth = authForPath(faceDef, path, method)
      const scopeFn = faceDef.scope.length >= 2 ? faceDef.scope(method, path) : faceDef.scope(method)
      buckets.get(group.key).push(
        buildOperation(method, path, op, {
          schemas,
          scope: scopeFn,
          auth
        })
      )
    }
  }

  for (const group of groupDefs) {
    for (const key of group.ops) {
      if (!placed.has(key)) {
        throw new Error(
          `docs-model grouping guard: face ${faceDef.key} group ${group.key} lists ${key}, which the spec no longer has`
        )
      }
    }
  }

  if (faceDef.include) {
    const builtIds = new Set(
      [...buckets.values()].flatMap((ops) => ops.map((o) => o.id))
    )
    for (const id of faceDef.include) {
      if (!builtIds.has(id)) {
        throw new Error(
          `docs-model coverage guard: included operation ${id} was not found under ${faceDef.prefix}`
        )
      }
    }
  }

  return {
    key: faceDef.key,
    label: faceDef.label,
    name: faceDef.name,
    baseUrl: API_HOST,
    prefix: faceDef.prefix,
    auth: faceDef.auth,
    ...(faceDef.notes?.length && { notes: faceDef.notes }),
    groups: groupDefs.map((g) => ({
      key: g.key,
      label: g.label,
      operations: buckets
        .get(g.key)
        .sort((a, b) => g.ops.indexOf(opKey(a.method, a.path)) - g.ops.indexOf(opKey(b.method, b.path)))
    }))
  }
}

const specs = new Map()
for (const faceDef of FACES) {
  if (!specs.has(faceDef.file)) {
    specs.set(faceDef.file, parseYaml(readFileSync(faceDef.file, 'utf8')))
  }
}

const model = { faces: FACES.map((f) => buildFace(f, specs)) }

const faceOpCount = (f) =>
  f.groups.reduce((m, g) => m + g.operations.length, 0)

for (const face of model.faces) {
  const want = EXPECTED_OPERATION_COUNTS[face.key]
  const got = faceOpCount(face)
  if (want !== got) {
    throw new Error(
      `docs-model coverage guard: face ${face.key} expected ${want} operations, built ${got}`
    )
  }
}

// A path the prefixes do not claim would vanish from the reference with every
// other guard still green, which is how five playtime operations shipped
// documented as catalog ones. Full-coverage applies to the two fully-published
// specs only — openapi.yaml is a first-party spec whose unclaimed prefixes must
// not trip this guard.
for (const file of [PUBLIC_SPEC, NEWS_SPEC, V2_SPEC]) {
  const claimed = new Set(
    model.faces
      .filter((f) => FACES.find((d) => d.key === f.key)?.file === file)
      .flatMap((f) => f.groups.flatMap((g) => g.operations.map((o) => o.path)))
  )
  for (const path of Object.keys(specs.get(file).paths || {})) {
    if (!claimed.has(path)) {
      throw new Error(
        `docs-model coverage guard: spec path ${path} belongs to no face — add one to FACES`
      )
    }
  }
}

const editDef = FACES.find((f) => f.key === 'edit')
const catalogSpec = specs.get(CATALOG_SPEC)
const namedEditOps = new Set([
  ...(editDef.include || []),
  ...EDIT_EXCLUDED_OPERATION_IDS
])
for (const [path, item] of Object.entries(catalogSpec.paths || {})) {
  if (!path.startsWith(editDef.prefix)) continue
  for (const method of METHODS) {
    const op = item[method]
    if (!op) continue
    if (!namedEditOps.has(op.operationId)) {
      throw new Error(
        `docs-model completeness guard: ${op.operationId} under ${editDef.prefix} is neither included nor excluded`
      )
    }
  }
}

const opCount = model.faces.reduce((n, f) => n + faceOpCount(f), 0)

const out = `/**
 * Auto-generated by scripts/sync-specs.mjs — do not edit by hand.
 * Run \`pnpm --filter developer sync:specs\` after the public specs change.
 *
 * Three Tier-A specs projected into the render-friendly DocsModel the
 * /docs/** reference pages consume, one entry per public face.
 */
import type { DocsModel } from '~~/shared/types/docs'

export const docsModel: DocsModel = ${JSON.stringify(model, null, 2)}
`

mkdirSync(dirname(OUTPUT), { recursive: true })
writeFileSync(OUTPUT, out)
const breakdown = model.faces
  .map((f) => `${f.key} ${faceOpCount(f)}`)
  .join(', ')
console.log(
  `Wrote docs model → app/generated/docs-model.ts (${model.faces.length} faces, ${opCount} operations: ${breakdown})`
)

const llmFiles = writeLlmArtifacts(model, PUBLIC_DIR)
console.log(
  `Wrote LLM artifacts → public/ (llms.txt, llms-full.txt, ${llmFiles - 2} route Markdown twins)`
)
