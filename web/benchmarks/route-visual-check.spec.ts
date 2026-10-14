import { test, expect } from '@playwright/test'

const routes = [
  'workspace',
  'operations',
  'terminal',
  'activity',
  'office',
  'knowledge',
  'staff',
  'brain',
  'settings',
]

test.describe('All 9 routes visual inspection', () => {
  for (const route of routes) {
    test(`renders ${route} cleanly in dark and light mode without error`, async ({ page }) => {
      const consoleErrors: string[] = []
      page.on('console', (msg) => {
        if (msg.type() === 'error') {
          consoleErrors.push(msg.text())
        }
      })

      // Navigate to route
      await page.goto(`/#${route}`)
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(300)

      // Ensure shell loaded
      await expect(page.locator('.shell')).toBeVisible()

      // Dark mode snapshot
      await page.evaluate(() => {
        document.documentElement.dataset.theme = 'dark'
      })
      await page.waitForTimeout(100)
      await page.screenshot({ path: `test-results/screenshots/${route}-dark.png`, fullPage: false })

      // Light mode snapshot
      await page.evaluate(() => {
        document.documentElement.dataset.theme = 'light'
      })
      await page.waitForTimeout(100)
      await page.screenshot({ path: `test-results/screenshots/${route}-light.png`, fullPage: false })

      // Verify no unexpected fatal React or rendering errors
      const fatalErrors = consoleErrors.filter(
        (err) =>
          err.includes('Uncaught') ||
          err.includes('Minified React error') ||
          err.includes('Cannot read properties of undefined')
      )
      expect(fatalErrors).toEqual([])
    })
  }
})
