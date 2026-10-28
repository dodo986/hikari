import { useEffect } from 'react'
import { navigate } from '../router'

const typing = (target: EventTarget | null): boolean => {
  const node = target as HTMLElement | null
  if (!node) return false
  return node.tagName === 'INPUT' || node.tagName === 'TEXTAREA' || node.isContentEditable
}

// Slash jumps to the library search, escape leaves a game page; a sheet keeps its escape.
export function useHotkeys(): void {
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === '/' && !typing(event.target)) {
        const search = document.querySelector<HTMLInputElement>('input[type="search"]')
        if (!search) return
        event.preventDefault()
        search.focus()
        search.select()
        return
      }
      if (event.key !== 'Escape' || typing(event.target)) return
      if (document.querySelector('dialog[open]')) return
      if (window.location.hash.includes('/game/')) navigate({ page: 'library' })
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
}
