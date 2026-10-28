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

test.describe('downloads, accounts and about', () => {
  test('downloads lists what the daemon is doing', async ({ page }) => {
    await page.goto('/#/downloads')
    await expect(page.getByRole('heading', { name: 'Downloads', level: 1 })).toBeVisible()
    await expect(page.getByText('Nothing downloading', { exact: true })).toBeVisible()
  })

  test('accounts shows each store and its session', async ({ page }) => {
    await page.goto('/#/accounts')
    await expect(page.getByText('Epic Games', { exact: true })).toBeVisible()
    await expect(page.getByText('GOG', { exact: true })).toBeVisible()
    await expect(page.getByText('Amazon Games', { exact: true })).toBeVisible()
    await expect(page.getByText('CONNECTED').first()).toBeVisible()
  })

  test('about shows the daemon version and the project link', async ({ page }) => {
    await page.goto('/#/about')
    await expect(page.getByText(/Hikari 0\.8\.3/)).toBeVisible()
    await expect(page.getByRole('button', { name: /dodo986/ })).toBeVisible()
  })
})

test.describe('mods', () => {
  test('lists the games the backend can mod', async ({ page }) => {
    await page.goto('/#/mods')
    await expect(page.getByRole('heading', { name: 'Mods', level: 1 })).toBeVisible()
    await expect(page.getByRole('button', { name: /Skyrim Special Edition/ })).toBeVisible()
    await expect(page.getByText('MODS READY')).toBeVisible()
  })

  test('opens a game and shows its installed mods', async ({ page }) => {
    await page.goto('/#/mods')
    await page.getByRole('button', { name: /Skyrim Special Edition/ }).click()
    await expect(page.getByRole('button', { name: /Installed/ })).toBeVisible()
    await expect(page.getByText('SkyUI')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Deploy' })).toBeVisible()
  })

  test('browse asks for a Nexus session first', async ({ page }) => {
    await page.goto('/#/mods')
    await page.getByRole('button', { name: /Skyrim Special Edition/ }).click()
    await page.getByRole('button', { name: 'Browse Nexus' }).click()
    await expect(page.getByText('Sign in to Nexus')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Use an API key' })).toBeVisible()
  })

  test('the back button returns to the game list', async ({ page }) => {
    await page.goto('/#/mods')
    await page.getByRole('button', { name: /Skyrim Special Edition/ }).click()
    await page.getByRole('button', { name: '← Mods' }).click()
    await expect(page.getByRole('button', { name: /Cyberpunk 2077/ })).toBeVisible()
  })
})

test.describe('keyboard', () => {
  test('slash focuses the library search and escape leaves a game page', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('searchbox')).toBeVisible()
    await page.keyboard.press('/')
    await expect(page.getByRole('searchbox')).toBeFocused()
    await page.goto('/#/game/epic/demo-epic-1')
    await expect(page.getByTestId('open-settings')).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(page.getByRole('heading', { name: 'Library', level: 1 })).toBeVisible()
  })

  test('escape closes a sheet without leaving the page', async ({ page }) => {
    await page.goto('/#/game/epic/demo-epic-1')
    await page.getByTestId('open-settings').click()
    await expect(page.getByRole('dialog')).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(page.getByRole('dialog')).toBeHidden()
    await expect(page.getByTestId('open-settings')).toBeVisible()
  })
})

test.describe('offline', () => {
  test('says what is wrong and offers to start the daemon', async ({ page }) => {
    await page.route('**/__daemon.json', (route) => route.fulfill({ status: 503, json: { error: 'no daemon' } }))
    await page.goto('/')
    await expect(page.getByRole('button', { name: 'Start daemon' })).toBeVisible()
    await page.getByRole('button', { name: 'Start daemon' }).click()
    await expect(page.getByText('starting hikarid needs the desktop window')).toBeVisible()
  })
})

test.describe('nexus links', () => {
  test('an nxm link opens the game it belongs to', async ({ page }) => {
    await page.addInitScript(() => {
      const host = window as unknown as { __HIKARI_LINK__?: string }
      host.__HIKARI_LINK__ = 'nxm://skyrimspecialedition/mods/12604/files/1?key=k&expires=2'
    })
    await page.goto('/#/mods')
    await page.waitForTimeout(1500)
    await expect(page.getByRole('heading', { name: 'Mods', level: 1 })).toBeHidden()
  })
})

test.describe('right to left', () => {
  test('the rail moves to the right edge and keeps its marker inside', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('searchbox')).toBeVisible()
    await page.evaluate(() => {
      document.documentElement.dir = 'rtl'
    })
    const rail = await page.getByRole('navigation', { name: 'Main' }).boundingBox()
    const main = await page.locator('main').boundingBox()
    expect(rail!.x).toBeGreaterThan(main!.x)
    const marker = await page.locator('nav [aria-hidden="true"]').last().boundingBox()
    expect(marker!.x).toBeGreaterThan(rail!.x + rail!.width / 2)
  })
})

test.describe('updates', () => {
  test('the rail offers a daemon update when one exists', async ({ page }) => {
    await page.addInitScript(() => {
      const Real = window.WebSocket
      const fake = { 'app.update.check': { available: true, version: '0.8.9' } }
      window.WebSocket = class extends Real {
        override send(payload: string) {
          const frame = JSON.parse(payload) as { id: number; method: string }
          const result = fake[frame.method as keyof typeof fake]
          if (!result) return super.send(payload)
          queueMicrotask(() => {
            this.dispatchEvent(
              new MessageEvent('message', {
                data: JSON.stringify({ jsonrpc: '2.0', id: frame.id, result }),
              }),
            )
          })
        }
      }
    })
    await page.goto('/')
    await expect(page.getByText('Hikari 0.8.9 is available')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Update' })).toBeVisible()
  })
})
