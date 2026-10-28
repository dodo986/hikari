import { useCallback } from 'react'

/**
 * Strings are keyed by their English text, exactly like I18n.tr in the QML
 * app, so a translation file only has to list the strings it changes.
 */
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

// Re-renders every subscriber when the language changes.
const listeners = new Set<() => void>()

export function onLanguageChange(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function changeLanguage(next: string, catalogue?: Record<string, string>): void {
  setLanguage(next, catalogue)
  for (const listener of listeners) listener()
}
