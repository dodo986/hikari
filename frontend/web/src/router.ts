export type PageName =
  | 'library'
  | 'store'
  | 'mods'
  | 'downloads'
  | 'accounts'
  | 'settings'
  | 'about'
  | 'game'

export interface Route {
  page: PageName
  store?: string
  id?: string
}

const PAGES: PageName[] = ['library', 'store', 'mods', 'downloads', 'accounts', 'settings', 'about']

export function parseRoute(hash: string): Route {
  const clean = hash.replace(/^#\/?/, '')
  const [head = '', store, id] = clean.split('/')
  if (head === 'game' && store && id) return { page: 'game', store, id: decodeURIComponent(id) }
  const page = PAGES.find((name) => name === head)
  return { page: page ?? 'library' }
}

export function href(route: Route): string {
  if (route.page === 'game' && route.store && route.id) {
    return `#/game/${route.store}/${encodeURIComponent(route.id)}`
  }
  return `#/${route.page}`
}

export function navigate(route: Route): void {
  window.location.hash = href(route)
}
