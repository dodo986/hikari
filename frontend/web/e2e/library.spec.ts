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
