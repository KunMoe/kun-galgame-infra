import docsZh from '../../i18n/docs-zh.json'

const catalog = docsZh as Record<string, string>

export const useDocsI18n = () => {
  const t = (text?: string): string => {
    if (!text) return ''
    return catalog[text] ?? text
  }
  return { t }
}
