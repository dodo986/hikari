import { useCallback } from 'react'

// Keyed by the English text, like I18n.tr, so a catalogue only lists what it changes.
type Replacements = Record<string, string | number>

const overrides: Record<string, string> = {}
let language = 'en'

export function setLanguage(next: string, catalogue: Record<string, string> = {}): void {
  language = next
  for (const key of Object.keys(overrides)) delete overrides[key]
  if (next === 'en') return
  Object.assign(overrides, catalogue)
}

export function t(text: string, values?: Replacements): string {
  const source = language === 'en' ? text : (overrides[text] ?? text)
  if (!values) return source
  return source.replace(/\{(\w+)\}/g, (match, name: string) =>
    name in values ? String(values[name]) : match,
  )
}

export function useT(): (text: string, values?: Replacements) => string {
  return useCallback((text: string, values?: Replacements) => t(text, values), [])
}
