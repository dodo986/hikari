import { defineConfig, devices } from '@playwright/test'

const RUNTIME = process.env.HIKARI_E2E_RUNTIME ?? '/tmp/hikari-e2e'

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1,
  reporter: [['list']],
  use: {
    baseURL: 'http://127.0.0.1:5173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    launchOptions: {
      // No downloaded browsers here: the system Chromium drives the tests.
      executablePath: process.env.HIKARI_CHROMIUM ?? '/usr/bin/chromium',
      args: ['--no-sandbox'],
    },
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'], viewport: { width: 1400, height: 900 } } }],
  webServer: {
    command: 'bash ../scripts/dev-web.sh',
    url: 'http://127.0.0.1:5173',
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    env: { HIKARI_DEV_RUNTIME: RUNTIME },
  },
})
