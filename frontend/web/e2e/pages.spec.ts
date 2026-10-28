import { expect, test } from '@playwright/test'

test.describe('game detail', () => {
  test('opens from a library tile and plays without leaving the page', async ({ page }) => {
    await page.goto('/')
    await page.getByTestId('tile').first().click()
    await expect(page.getByRole('button', { name: /Play now|Install game/ })).toBeVisible()
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
    await expect(page.getByText('Game details')).toBeVisible()
  })

  test('the settings sheet writes launch extras through the daemon', async ({ page }) => {
    await page.goto('/#/game/epic/demo-epic-1')
    await page.getByTestId('open-settings').click()
    const sheet = page.getByRole('dialog')
    await expect(sheet).toBeVisible()
    await expect(sheet.getByText('LAUNCH OPTIONS', { exact: true })).toBeVisible()
    await sheet.getByRole('button', { name: 'Close' }).click()
    await expect(sheet).toBeHidden()
  })

  test('the back button returns to the library', async ({ page }) => {
    await page.goto('/#/game/epic/demo-epic-1')
    await page.getByRole('button', { name: '← Library' }).click()
    await expect(page.getByRole('heading', { name: 'Library', level: 1 })).toBeVisible()
  })
})

test.describe('settings page', () => {
  test('lists the engines the daemon found', async ({ page }) => {
    await page.goto('/#/settings')
    await expect(page.getByRole('heading', { name: 'Settings', level: 1 })).toBeVisible()
    await expect(page.getByText('legendary', { exact: true })).toBeVisible()
    await expect(page.getByText('hikari 0.8.3')).toBeVisible()
  })

  test('the artwork key is write only', async ({ page }) => {
    await page.goto('/#/settings')
    const field = page.getByLabel('SteamGridDB key')
    await expect(field).toHaveAttribute('type', 'password')
  })
})
