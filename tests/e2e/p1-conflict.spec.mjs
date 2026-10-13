import { test, expect } from '../helpers/p1-session.mjs';
import { credentials, login } from '../helpers/p1-session.mjs';
import { upload, extract, snapshot } from '../helpers/api.mjs';

test('P1-CAS-001 two account browsers collide without automatic overwrite and retain losing draft', async ({ page, browser, request }, testInfo) => {
  const video = await upload(request);
  const { frames } = await extract(request, video.id);
  const frame = frames[0];
  const other = await browser.newContext({ baseURL: process.env.P0_BASE_URL });
  await login(other.request, 'p1_account_two', credentials.password);
  const second = await other.newPage();
  try {
    for (const view of [page, second]) {
      await view.goto('/');
      await view.getByTestId('video-select').selectOption(video.id);
      await view.getByTestId(`frame-item-${frame.id}`).click();
      await expect(view.getByTestId('annotation-canvas')).toBeVisible();
      await view.getByTestId('draw-mode').click();
    }
    let arrived = 0;
    let release;
    const barrier = new Promise(resolve => { release = resolve; });
    const responses = [];
    for (const view of [page, second]) await view.route(`**/api/v1/frames/${frame.id}/annotations`, async route => {
      if (route.request().method() !== 'PUT') return route.continue();
      arrived++;
      if (arrived === 2) release();
      await barrier;
      const response = await route.fetch();
      responses.push({ status: response.status(), view });
      await route.fulfill({ response });
    });
    const draw = async (view, x) => {
      const bounds = await view.getByTestId('annotation-canvas').locator('.konvajs-content').boundingBox();
      await view.mouse.move(bounds.x + bounds.width * x, bounds.y + bounds.height * 0.2);
      await view.mouse.down();
      await view.mouse.move(bounds.x + bounds.width * (x + 0.15), bounds.y + bounds.height * 0.4, { steps: 6 });
      await view.mouse.up();
    };
    await Promise.all([draw(page, 0.1), draw(second, 0.6)]);
    await expect.poll(() => responses.length).toBe(2);
    expect(responses.map(response => response.status).sort()).toEqual([200, 409]);
    const loser = responses.find(response => response.status === 409).view;
    await expect(loser.getByTestId('save-state')).toContainText('冲突');
    const committed = await snapshot(request, frame.id);
    expect(committed.annotations).toHaveLength(1);
    expect(committed.revision).toBe(1);
    const drafts = await loser.evaluate(() => Object.keys(localStorage).filter(key => key.includes('draft')).map(key => ({ key, value: localStorage.getItem(key) })));
    expect(drafts.some(draft => draft.value.includes(frame.id))).toBe(true);
    await loser.waitForTimeout(2000);
    expect(await snapshot(request, frame.id)).toEqual(committed);
    await loser.screenshot({ path: testInfo.outputPath('two-account-conflict.png'), fullPage: true });
  } finally { await other.close(); }
});
