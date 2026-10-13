import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  timeout: 120_000,
  expect: { timeout: 15_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  forbidOnly: !!process.env.CI,
  outputDir: process.env.P0_TEST_OUTPUT || 'test-results',
  reporter: [['list'], ['html', { open: 'never', outputFolder: process.env.P0_REPORT_OUTPUT || 'playwright-report' }]],
  use: {
    baseURL: process.env.P0_BASE_URL || 'http://127.0.0.1:8180',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'api', testMatch: /integration\/.*\.spec\.mjs/ },
    { name: 'chromium', testMatch: /e2e\/.*\.spec\.mjs/, use: { browserName: 'chromium',
      channel: process.env.P01_BROWSER_CHANNEL || undefined, viewport: { width: 1280, height: 900 } } },
  ],
});
