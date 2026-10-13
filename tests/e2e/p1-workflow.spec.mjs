import { test, expect } from '@playwright/test';
import { randomUUID } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { credentials, login } from '../helpers/p1-session.mjs';
import { upload, extract } from '../helpers/api.mjs';

test('P1-UI/AUTH/HELP-001 browser registration, login, three viewports and static help fallback', async ({ page }, testInfo) => {
  await page.goto('/');
  await page.getByRole('button', { name: '使用指南', exact: true }).click();
  await expect(page.getByRole('navigation', { name: '手册章节' })).toBeVisible();
  await page.getByRole('button', { name: '收起使用指南' }).click();
  await page.getByRole('button', { name: '注册账号', exact: true }).click();
  await page.getByLabel('用户名', { exact: true }).fill(`browser_${randomUUID().slice(0, 8)}`);
  await page.getByLabel('密码', { exact: true }).fill(credentials.password);
  await page.getByLabel('邀请码', { exact: true }).fill('synthetic-ci-invite');
  await page.getByRole('button', { name: '创建账号', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('账号已创建');
  await expect(page.getByRole('button', { name: '退出登录' })).toHaveCount(0);
  await page.getByRole('button', { name: '登录工作区', exact: true }).click();
  await expect(page.getByRole('button', { name: '退出登录' })).toBeVisible();
  for (const [width, height] of [[1440, 900], [1280, 720], [390, 844]]) {
    await page.setViewportSize({ width, height });
    await page.screenshot({ path: testInfo.outputPath(`p1-${width}x${height}.png`), fullPage: true });
  }
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.getByRole('button', { name: '使用指南', exact: true }).click();
  await page.getByLabel('操作问题').fill('如何处理版本冲突？');
  await page.getByRole('button', { name: '依据手册回答', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('完整手册仍可查阅');
  await expect(page.getByRole('navigation', { name: '手册章节' })).toBeVisible();
  await page.getByRole('button', { name: '收起使用指南' }).click();
  await page.getByRole('button', { name: '退出登录' }).click();
  await expect(page.getByRole('button', { name: '登录工作区' })).toBeVisible();
});

test('P1-LOWCONC-001 five independent browser sessions, three accounts, ten minute mixed workload', async ({ browser }, testInfo) => {
  test.setTimeout(680_000);
  const contexts = [];
  const records = [];
  const tokenValues = new Set();
  const started = Date.now();
  try {
    const users = [credentials.username, 'p1_account_two', 'p1_account_three', credentials.username, 'p1_account_three'];
    for (const username of users) {
      const context = await browser.newContext({ baseURL: process.env.P0_BASE_URL }); contexts.push(context);
      const session = await login(context.request, username);
      await context.setExtraHTTPHeaders({ Origin: process.env.P0_BASE_URL, 'X-CSRF-Token': session.csrfToken });
      tokenValues.add((await context.cookies()).find(cookie => cookie.name === 'cvte_session').value);
      const page = await context.newPage();
      await page.goto(process.env.P0_BASE_URL);
      await expect(page.getByRole('button', { name: '退出登录' })).toBeVisible();
      records.push({ context, page, username, headers: { Origin: process.env.P0_BASE_URL, 'X-CSRF-Token': session.csrfToken },
        reads: 0, saves: 0, exports: 0, conflicts: 0, latencies: [] });
    }
    expect(tokenValues.size).toBe(5);
    const first = records[0];
    const video = await upload(first.context.request);
    const { frames } = await extract(first.context.request, video.id, { intervalSeconds: 0.5 });
    expect(frames).toHaveLength(6);
    await Promise.all(records.map(async (record, index) => {
      await record.page.getByTestId('video-select').selectOption(video.id);
      await record.page.getByTestId(`frame-item-${frames[index].id}`).click();
      await expect(record.page.getByTestId('annotation-canvas')).toBeVisible();
    }));
    const workloadStart = Date.now();
    await Promise.all(records.map(async (record, index) => {
      const frame = frames[index];
      const annotation = { id: randomUUID(), bbox: [20 + index * 30, 20, 20, 20], source: 'manual' };
      while (Date.now() - workloadStart < 600_000) {
        const begin = Date.now();
        const current = await record.context.request.get(`/api/v1/frames/${frame.id}/annotations`);
        expect(current.status()).toBe(200);
        const state = await current.json(); record.reads++;
        const saved = await record.context.request.put(`/api/v1/frames/${frame.id}/annotations`, {
          headers: record.headers, data: { expectedRevision: state.revision,
            status: record.saves % 2 ? 'completed' : 'in_progress', annotations: [annotation] } });
        if (saved.status() === 409) record.conflicts++;
        else { expect(saved.status()).toBe(200); record.saves++; }
        record.latencies.push(Date.now() - begin);
        expect((await record.context.request.get(frame.imageUrl)).status()).toBe(200);
        if (record.saves % 2 === 0) {
          const archive = await record.context.request.post('/api/v1/exports', { headers: record.headers,
            data: { frameIds: [frame.id] } });
          expect(archive.status()).toBe(200); record.exports++;
        }
        if (index === 0 && record.saves === 1) {
          const aiFrame = frames[5];
          const aiBefore = await record.context.request.get(`/api/v1/frames/${aiFrame.id}/annotations`);
          const aiQueued = await record.context.request.post(`/api/v1/frames/${aiFrame.id}/preannotations`, {
            headers: { ...record.headers, 'Idempotency-Key': randomUUID() },
            data: { expectedRevision: (await aiBefore.json()).revision } });
          expect(aiQueued.status()).toBe(202);
          record.concurrentAIJobs = 1;
        }
        if (index === 0 && record.saves === 2) {
          let review;
          await expect.poll(async () => {
            const reviewed = await record.context.request.get(`/api/v1/frames/${frames[5].id}/review`);
            expect(reviewed.status()).toBe(200); review = await reviewed.json();
            return review.currentRun?.status;
          }, { timeout: 40_000 }).toBe('completed');
          for (const candidate of review.candidates) {
            const resolved = await record.context.request.put(`/api/v1/frames/${frames[5].id}/review`, {
              headers: record.headers, data: { expectedRevision: review.revision,
                decisions: [{ candidateId: candidate.id, decision: 'rejected' }], issueResolutions: [] } });
            expect(resolved.status()).toBe(200);
            review = await resolved.json();
          }
          const unresolvedIssues = review.issues.filter(issue => !issue.resolved);
          if (unresolvedIssues.length) {
            const resolved = await record.context.request.put(`/api/v1/frames/${frames[5].id}/review`, {
              headers: record.headers, data: { expectedRevision: review.revision, decisions: [],
                issueResolutions: unresolvedIssues.map(issue => ({ id: issue.id, resolution: 'manual_checked' })) } });
            expect(resolved.status()).toBe(200);
          }
          record.reviews = 1;
        }
        await record.page.waitForTimeout(5000);
      }
      const final = await record.context.request.get(`/api/v1/frames/${frame.id}/annotations`);
      expect((await final.json()).annotations).toEqual([annotation]);
    }));
    expect(Date.now() - workloadStart).toBeGreaterThanOrEqual(600_000);
    await records[0].page.screenshot({ path: testInfo.outputPath('five-session-workload.png'), fullPage: true });
    const evidence = { sourceSHA: process.env.P0_EXPECTED_SHA, observedAt: new Date().toISOString(),
      elapsedMs: Date.now() - started, contexts: 5, accounts: 3, distinctSessions: 5,
      workloadMs: Date.now() - workloadStart, records: records.map(({ username, reads, saves, exports, conflicts, latencies, reviews = 0, concurrentAIJobs = 0 }) => {
        latencies.sort((a, b) => a - b);
        return { username, reads, saves, exports, conflicts, reviews, concurrentAIJobs, samples: latencies.length,
          readAndSaveP95Ms: latencies[Math.ceil(latencies.length * 0.95) - 1] };
      }) };
    fs.writeFileSync(path.join(process.env.P1_STACK_EVIDENCE, 'low-concurrency-evidence.json'), JSON.stringify(evidence, null, 2));
  } finally { for (const context of contexts) await context.close(); }
});

test('P1-AUTH-001 failed browser logout shows a revocation warning after clearing local identity', async ({ page }) => {
  await login(page.request);
  await page.goto('/');
  await expect(page.getByRole('button', { name: '退出登录' })).toBeVisible();
  await page.route('**/api/v1/auth/logout', route => route.abort('failed'));
  await page.getByRole('button', { name: '退出登录' }).click();
  await expect(page.getByRole('button', { name: '登录工作区' })).toBeVisible();
  await expect(page.getByRole('alert')).toContainText('退出请求未确认');
});
