import { describe, expect, it } from 'vitest'
import { href, parseRoute } from '../src/router'

describe('route parsing', () => {
  it('reads the page from the hash', () => {
    expect(parseRoute('#/library')).toEqual({ page: 'library' })
    expect(parseRoute('#/settings')).toEqual({ page: 'settings' })
  })

  it('reads a game route with its store and id', () => {
    expect(parseRoute('#/game/epic/nativegame')).toEqual({ page: 'game', store: 'epic', id: 'nativegame' })
    expect(parseRoute('#/game/local/a%20b')).toEqual({ page: 'game', store: 'local', id: 'a b' })
  })

  it('falls back to the library for anything unknown', () => {
    expect(parseRoute('')).toEqual({ page: 'library' })
    expect(parseRoute('#/nope')).toEqual({ page: 'library' })
    expect(parseRoute('#/mods')).toEqual({ page: 'mods' })
  })

  it('round trips through href', () => {
    const route = { page: 'game' as const, store: 'gog', id: 'a/b' }
    expect(parseRoute(href(route))).toEqual(route)
    expect(href({ page: 'downloads' })).toBe('#/downloads')
  })
})
