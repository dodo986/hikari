import { describe, expect, it } from 'vitest'
import { gameForNxm, parseNxm } from '../src/lib/nxm'

const games = [{ id: 'skyrim_se', nexus_domain: 'skyrimspecialedition' }] as never

describe('nxm links', () => {
  it('reads the domain, mod and file', () => {
    const link = parseNxm('nxm://skyrimspecialedition/mods/12604/files/1?key=k&expires=2')
    expect(link?.domain).toBe('skyrimspecialedition')
    expect(link?.modId).toBe(12604)
    expect(link?.fileId).toBe(1)
  })

  it('keeps the download key in the url it passes on', () => {
    const raw = 'nxm://skyrimspecialedition/mods/12604/files/1?key=abc&expires=2'
    expect(parseNxm(raw)?.url).toBe(raw)
  })

  it('rejects anything that is not a mod file link', () => {
    expect(parseNxm('https://nexusmods.com/mods/1')).toBeNull()
    expect(parseNxm('nxm://skyrimspecialedition/mods/1')).toBeNull()
    expect(parseNxm('nxm://')).toBeNull()
  })

  it('maps a domain to the backend game key', () => {
    const link = parseNxm('nxm://skyrimspecialedition/mods/1/files/2')!
    expect(gameForNxm(link, games)).toBe('skyrim_se')
    expect(gameForNxm({ ...link, domain: 'cyberpunk2077' }, games)).toBeNull()
  })
})
