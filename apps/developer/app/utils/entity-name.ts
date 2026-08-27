const LOCALE_ORDER = ['zh-Hans', 'zh-Hant', 'zh', 'ja', 'en']

const str = (v: unknown): string | null =>
  typeof v === 'string' && v ? v : null

const fromLocalized = (v: unknown): string | null => {
  if (!v || typeof v !== 'object') return null
  const map = v as Record<string, unknown>
  const at = (locale: string): string | null => {
    const row = map[locale]
    return row && typeof row === 'object'
      ? str((row as Record<string, unknown>).value)
      : null
  }
  for (const locale of LOCALE_ORDER) {
    const value = at(locale)
    if (value) return value
  }
  for (const locale of Object.keys(map)) {
    const value = at(locale)
    if (value) return value
  }
  return null
}

export const entityName = (data: Record<string, unknown>): string | null =>
  fromLocalized(data.localized) ??
  str(data.display_name) ??
  str(data.latin) ??
  str(data.name)
