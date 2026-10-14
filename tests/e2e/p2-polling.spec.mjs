import { test, expect } from '@playwright/test';
import { ids, mockResponsiveApi } from '../helpers/p2-responsive-fixture.mjs';

const stats = {
  candidates: 1, pending: 1, accepted: 0, adjusted: 0, rejected: 0,
  unresolvedRiskCandidates: 1, unresolvedRiskImages: 1, manualAdded: 0, failedImages: 0,
  unresolvedRiskFrameIds: [ids.frame],
  rates: { acceptedUnchanged: null, adjusted: null, rejected: null,
    suspiciousCandidates: 1, suspiciousImages: 1, reviewed: 0 },
};

async function openPollingEditor(page) {
  const calls = await mockResponsiveApi(page);
  let hold = false;
  let pending;
  let status = 200;
  await page.route(`**/api/v1/videos/${ids.video}/review-stats?*`, async route => {
    if (hold) {
      hold = false;
      await new Promise(resolve => { pending = resolve; });
    }
    await route.fulfill({ status, contentType: 'application/json',
      body: JSON.stringify(status === 200 ? stats : { code: 'SYNTHETIC_OUTAGE', message: '合成统计服务暂时不可用' }) });
  });
  await page.route(`**/api/v1/videos/${ids.video}/frames?*`, route => route.fulfill({
    contentType: 'application/json', body: JSON.stringify({ frames: [ids.frame, 'synthetic-other-frame'].map((id, index) => ({
      id, videoId: ids.video, batchId: ids.batch, timestampMs: 1000 + index * 1000,
      width: 640, height: 360, imageUrl: '/media/p2-synthetic.svg', status: 'in_progress', revision: 7,
    })) }),
  }));
  await page.goto('/');
  await page.getByTestId('video-select').selectOption(ids.video);
  await page.getByTestId('annotation-canvas').locator('canvas').first().waitFor();
  await expect(page.locator('.review-stats summary')).toContainText('已决策 0/1');
  await expect(page.getByTestId('save-state')).toHaveText('已保存');
  return {
    calls,
    pauseNext: (nextStatus = 200) => { status = nextStatus; pending = undefined; hold = true; },
    waitPending: () => expect.poll(() => !!pending).toBe(true),
    release: () => { pending?.(); pending = undefined; },
  };
}

test('P2-POLL-001 unchanged background refresh preserves counters, risk filtering and expanded content', async ({ page }) => {
  const control = await openPollingEditor(page);
  const summary = page.locator('.review-stats summary');
  const filter = page.getByRole('checkbox', { name: '仅看需复核图片', exact: true });
  await summary.click();
  await filter.check();
  await expect(page.locator('.frame-list button')).toHaveCount(1);
  const frameNode = await page.getByTestId(`frame-item-${ids.frame}`).elementHandle();
  const progressNode = await page.locator('.review-stats progress').elementHandle();
  control.pauseNext();
  try {
    await control.waitPending();
    expect(await summary.textContent()).toContain('已决策 0/1');
    expect(await filter.isChecked()).toBe(true);
    expect(await filter.isEnabled()).toBe(true);
    expect(await page.locator('.frame-list button').count()).toBe(1);
    expect(await frameNode.evaluate(node => node.isConnected)).toBe(true);
    expect(await progressNode.evaluate(node => node.isConnected)).toBe(true);
    expect(await page.locator('.review-stats').evaluate(node => node.open)).toBe(true);
  } finally { control.release(); }
  await expect(summary).toContainText('已决策 0/1');
  expect(control.calls.writes).toEqual([]);
});

test('P2-POLL-002 a real refresh failure stays visible during retry and recovery restores risk filtering', async ({ page }) => {
  const control = await openPollingEditor(page);
  const filter = page.getByRole('checkbox', { name: '仅看需复核图片', exact: true });
  await filter.check();
  control.pauseNext(503);
  await control.waitPending();
  control.release();
  const alert = page.getByRole('alert').filter({ hasText: '复核统计暂不可用' });
  await expect(alert).toBeVisible();
  await expect(filter).toBeDisabled();
  const errorNode = await alert.elementHandle();
  control.pauseNext(200);
  try {
    await control.waitPending();
    expect(await errorNode.evaluate(node => node.isConnected)).toBe(true);
    expect(await alert.isVisible()).toBe(true);
    expect(await filter.isEnabled()).toBe(false);
  } finally { control.release(); }
  await expect(alert).toHaveCount(0);
  await expect(page.locator('.review-stats summary')).toContainText('已决策 0/1');
  await expect(filter).toBeChecked();
  await expect(filter).toBeEnabled();
  await expect(page.locator('.frame-list button')).toHaveCount(1);
  expect(control.calls.writes).toEqual([]);
});
