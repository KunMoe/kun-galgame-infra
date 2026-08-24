// Thin TypeScript client for NextMoe API v2. Compile-checked by the portal.

export const v2Client = (base: string, key?: string) => {
  const headers: Record<string, string> = {}
  if (key) headers.Authorization = `Bearer ${key}`

  const get = async (path: string, query?: Record<string, string>) => {
    const url = new URL(path, base.endsWith('/') ? base : base + '/')
    if (query) {
      for (const [k, v] of Object.entries(query)) {
        if (v) url.searchParams.set(k, v)
      }
    }
    const res = await fetch(url, { headers })
    const body: unknown = await res.json().catch(() => null)
    return { status: res.status, body }
  }

  return {
    problems: () => get('/v2/problems'),
    vocabulary: (name: string) => get(`/v2/vocabularies/${name}`),
    work: (id: string) => get(`/v2/catalog/works/${id}`),
    search: (q: string) => get('/v2/catalog/search', { q, object: 'work' })
  }
}
