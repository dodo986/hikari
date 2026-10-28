import { expect, test } from '@playwright/test'

test.describe('library', () => {
  test('renders the installed games from the daemon', async ({ page }) => {
    await page.goto('/')
    const tiles = page.getByTestId('tile')
    await expect(tiles.first()).toBeVisible()
    await expect(tiles).toHaveCount(8)
    await expect(page.getByRole('heading', { name: 'Library', level: 1 })).toBeVisible()
  })

  test('search narrows the grid', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByTestId('tile')).toHaveCount(8)
    await page.getByRole('searchbox').fill('cyber')
    await expect(page.getByTestId('tile')).toHaveCount(1)
    await expect(page.getByTestId('tile')).toHaveAttribute('data-title', 'Cyberpunk')
  })

  test('store chips filter by source', async ({ page }) => {
    await page.goto('/')
    await page.getByTestId('chip-gog').click()
    await expect(page.getByTestId('tile').first()).toBeVisible()
    const stores = await page.getByTestId('tile').evaluateAll((nodes) =>
      nodes.map((node) => node.getAttribute('data-store')),
    )
    expect(stores.length).toBeGreaterThan(0)
    expect(stores.every((store) => store === 'gog')).toBe(true)
  })

  test('the store page shows games that are not installed', async ({ page }) => {
    await page.goto('/#/store')
    await expect(page.getByRole('heading', { name: 'Store', level: 1 })).toBeVisible()
    await expect(page.getByTestId('tile')).toHaveCount(5)
  })

  test('cover art is served through the daemon bridge', async ({ page }) => {
    await page.goto('/')
    const image = page.locator('[data-testid=tile] img').first()
    await expect(image).toBeVisible()
    const loaded = await image.evaluate((node) => (node as HTMLImageElement).naturalWidth > 0)
    expect(loaded).toBe(true)
  })
})

test.describe('grid motion', () => {
  const games = [
    { store: 'gog', id: 'a', title: 'Alpha', installed: true, platform: 'linux', last_played: '2020-01-01T00:00:00Z' },
    { store: 'gog', id: 'b', title: 'Beta', installed: true, platform: 'linux', last_played: '2025-01-01T00:00:00Z' },
    { store: 'gog', id: 'c', title: 'Gamma', installed: true, platform: 'linux' },
  ]

  const stubLibrary = async (page: import('@playwright/test').Page, list: unknown = games) => {
    await page.addInitScript((list: unknown) => {
      const Real = window.WebSocket
      window.WebSocket = class extends Real {
        override send(payload: string) {
          const frame = JSON.parse(payload) as { id: number; method: string }
          if (frame.method !== 'library.list') return super.send(payload)
          queueMicrotask(() => {
            this.dispatchEvent(
              new MessageEvent('message', {
                data: JSON.stringify({ jsonrpc: '2.0', id: frame.id, result: { games: list } }),
              }),
            )
          })
        }
      }
    }, list)
  }

  test('a reorder moves the tiles instead of snapping', async ({ page }) => {
    await stubLibrary(page)
    await page.goto('/')
    await expect(page.getByTestId('tile')).toHaveCount(3)
    await page.waitForTimeout(700)
    const beta = page.locator('[data-testid=tile][data-title="Beta"]')
    const betaBefore = await beta.boundingBox()
    await expect(page.getByTestId('tile').first()).toHaveAttribute('data-title', 'Alpha')
    await page.getByRole('button', { name: 'Playing first' }).click()
    await page.waitForTimeout(600)
    const betaAfter = await beta.boundingBox()
    await expect(page.getByTestId('tile').first()).toHaveAttribute('data-title', 'Beta')
    expect(betaAfter!.x).toBeLessThan(betaBefore!.x - 100)
    await expect(page.getByTestId('tile-leaving')).toHaveCount(0)
  })

  test('widening the window adds columns and keeps every tile placed', async ({ page }) => {
    await stubLibrary(page)
    await page.setViewportSize({ width: 700, height: 900 })
    await page.goto('/')
    await expect(page.getByTestId('tile')).toHaveCount(3)
    const narrow = await page.locator('[data-columns]').getAttribute('data-columns')
    await page.setViewportSize({ width: 1400, height: 900 })
    await page.waitForTimeout(600)
    const wide = await page.locator('[data-columns]').getAttribute('data-columns')
    expect(Number(wide)).toBeGreaterThan(Number(narrow))
    const boxes = await page.getByTestId('tile').evaluateAll((nodes) =>
      nodes.map((node) => node.getBoundingClientRect().width),
    )
    expect(boxes.every((width) => width > 200)).toBe(true)
  })
})
