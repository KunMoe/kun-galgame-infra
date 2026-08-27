export default defineEventHandler(async (event): Promise<unknown> => {
  assertMethod(event, 'GET')
  const raw = event.context.params?.path ?? ''
  const path = new URL(raw, 'http://relay.local/').pathname.slice(1)
  const allowed =
    path.startsWith('v1/store/') ||
    path.startsWith('v2/catalog/') ||
    path === 'v2/catalog' ||
    path.startsWith('v2/news/') ||
    path === 'v2/news' ||
    path.startsWith('v2/store/') ||
    path.startsWith('v2/problems') ||
    path.startsWith('v2/vocabularies')
  if (!allowed) {
    setResponseStatus(event, 404)
    return {
      code: 404,
      message: 'only /v1/store/* and /v2 public reads are relayed'
    }
  }
  const base = useRuntimeConfig(event).nextmoeApiBase
  const query = getQuery(event)
  const auth = getHeader(event, 'authorization')
  try {
    return await $fetch<unknown>(`${base}/${path}` as string, {
      query,
      headers: auth ? { Authorization: auth } : {}
    })
  } catch (e) {
    const err = e as { statusCode?: number; data?: unknown }
    setResponseStatus(event, err.statusCode ?? 502)
    return err.data ?? { code: -1, message: 'upstream unreachable' }
  }
})
