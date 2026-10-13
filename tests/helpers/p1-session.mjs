import { test as base, expect } from '@playwright/test';
export { expect };
export const credentials = { username: 'p1_browser_fixture', password: 'synthetic-browser-password-2026' };
export async function login(request, username = credentials.username, password = credentials.password) {
  const response = await request.post('/api/v1/auth/login', { data: { username, password },
    headers: { Origin: process.env.P0_BASE_URL } });
  expect(response.status()).toBe(200);
  return response.json();
}
let sharedIdentity;
async function identity(playwright) {
  if (!sharedIdentity) sharedIdentity = (async () => {
    const context = await playwright.request.newContext({ baseURL: process.env.P0_BASE_URL });
    const session = await login(context);
    const state = await context.storageState();
    await context.dispose();
    return { session, state };
  })();
  return sharedIdentity;
}
export const test = base.extend({
  request: async ({ playwright }, use) => {
    const { session, state } = await identity(playwright);
    const authenticated = await playwright.request.newContext({ baseURL: process.env.P0_BASE_URL, storageState: state,
      extraHTTPHeaders: { Origin: process.env.P0_BASE_URL, 'X-CSRF-Token': session.csrfToken } });
    await use(authenticated);
    await authenticated.dispose();
  },
  page: async ({ browser, playwright }, use) => {
    const { state } = await identity(playwright);
    const context = await browser.newContext({ storageState: state, baseURL: process.env.P0_BASE_URL });
    const page = await context.newPage();
    await use(page);
    await context.close();
  },
});
