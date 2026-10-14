import { test, expect } from '@playwright/test';
import { assertReachable, assertResponsiveGeometry, captureResponsiveScene, doubleTextSize } from '../helpers/p2-responsive-geometry.mjs';
import { ids, loadSyntheticEditor, longError, longName, mockResponsiveApi } from '../helpers/p2-responsive-fixture.mjs';

// Deliberately independent from the authenticated real-stack P1 fixture.
// This file verifies React rendering against synthetic routes, not backend correctness or model quality.
const sizes = [[320, 568], [360, 720], [390, 900], [768, 568], [1024, 720], [1280, 568], [1440, 900], [1920, 900]];
test.describe('P2-UI-001 / P2-TEST-001 synthetic responsive acceptance', () => {
  for (const [width, height] of sizes) {
    test(`login and registration ${width}x${height}`, async ({ page }, info) => {
      await page.setViewportSize({ width, height });
      await mockResponsiveApi(page, { authenticated: false });
      await page.goto('/');
      await expect(page.getByRole('button', { name: '登录工作区', exact: true })).toBeVisible();
      await assertResponsiveGeometry(page, 'login');
      await captureResponsiveScene(page, info, 'login', 'unauthenticated_login');
      await assertReachable(page.getByRole('button', { name: '登录工作区', exact: true }));
      await page.getByLabel('用户名', { exact: true }).fill('synthetic_account');
      await page.getByLabel('密码', { exact: true }).fill('synthetic-password-only');
      await page.getByRole('button', { name: '登录工作区', exact: true }).click();
      await expect(page.getByRole('alert')).toContainText(longError);
      await assertResponsiveGeometry(page, 'login long error');
      await page.getByRole('button', { name: '注册账号', exact: true }).click();
      await page.getByLabel('用户名', { exact: true }).fill('synthetic_account');
      await page.getByLabel('密码', { exact: true }).fill('synthetic-password-only');
      await page.getByLabel('邀请码', { exact: true }).fill('synthetic-invite-only');
      await page.getByRole('button', { name: '创建账号', exact: true }).click();
      await expect(page.getByRole('alert')).toContainText(longError);
      await assertResponsiveGeometry(page, 'registration long error');
      await assertReachable(page.getByRole('button', { name: '创建账号', exact: true }));
      await captureResponsiveScene(page, info, 'registration', 'registration_with_synthetic_service_error');
    });

    test(`loaded editor review and help ${width}x${height}`, async ({ page }, info) => {
      await page.setViewportSize({ width, height });
      const calls = await loadSyntheticEditor(page);
      await expect(page.getByTestId('candidate-synthetic-candidate')).toBeVisible();
      await assertResponsiveGeometry(page, 'long names and review');
      await captureResponsiveScene(page, info, 'editor-review', 'loaded_long_names_pending_risk_candidate');
      for (const id of ['draw-mode', 'select-mode', 'complete-frame', 'export-zip']) await assertReachable(page.getByTestId(id));
      await page.getByRole('button', { name: '使用指南', exact: true }).click();
      await expect(page.getByRole('navigation', { name: '手册章节' })).toBeVisible();
      await page.getByLabel('操作问题').fill('如何处理版本冲突？');
      await page.getByRole('button', { name: '依据手册回答', exact: true }).click();
      await expect(page.getByRole('alert').filter({ hasText: '完整手册仍可查阅' })).toBeVisible();
      await assertResponsiveGeometry(page, 'help and offline question');
      for (const id of ['complete-frame', 'export-zip']) await assertReachable(page.getByTestId(id));
      await assertReachable(page.getByRole('button', { name: '收起使用指南', exact: true }));
      expect(calls.questions).toBe(1);
      expect(calls.writes).toEqual([]);
      expect(calls.unexpected).toEqual([]);
      await captureResponsiveScene(page, info, 'editor-help', 'loaded_editor_help_question_failure');
    });
  }

  for (const [width, height] of [[320, 568], [768, 720], [1280, 900]]) {
    test(`empty and offline library ${width}x${height}`, async ({ page }, info) => {
      await page.setViewportSize({ width, height });
      await mockResponsiveApi(page, { empty: true });
      await page.goto('/');
      await expect(page.getByText('还没有视频，先上传或导入。', { exact: true })).toBeVisible();
      await assertResponsiveGeometry(page, 'empty library');
      await captureResponsiveScene(page, info, 'empty-library', 'authenticated_empty_library');
      await page.route('**/api/v1/videos', route => route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 'SYNTHETIC_OFFLINE', message: longError }) }));
      await page.getByRole('button', { name: '刷新', exact: true }).click();
      await expect(page.locator('.library').getByRole('alert')).toContainText(longError);
      await assertResponsiveGeometry(page, 'offline empty workspace');
      await assertReachable(page.getByRole('button', { name: '刷新', exact: true }));
    });

    test(`200 percent text ${width}x${height}`, async ({ page }, info) => {
      await page.setViewportSize({ width, height });
      await loadSyntheticEditor(page);
      await page.getByRole('button', { name: '使用指南', exact: true }).click();
      await expect(page.getByRole('navigation', { name: '手册章节' })).toBeVisible();
      const before = await page.getByTestId('draw-mode').evaluate(element => parseFloat(getComputedStyle(element).fontSize));
      await doubleTextSize(page);
      const after = await page.getByTestId('draw-mode').evaluate(element => parseFloat(getComputedStyle(element).fontSize));
      expect(after).toBeCloseTo(before * 2, 2);
      await assertResponsiveGeometry(page, '200 percent text with help');
      for (const id of ['draw-mode', 'complete-frame', 'export-zip']) await assertReachable(page.getByTestId(id));
      await captureResponsiveScene(page, info, 'text200', 'loaded_editor_help_text200');
    });
  }

  for (const [width, height] of [[320, 568], [768, 720], [1280, 900]]) {
    test(`expanded quality report and failure ${width}x${height}`, async ({ page }, info) => {
      await page.setViewportSize({ width, height });
      const calls = await loadSyntheticEditor(page, { review: false, completed: true });
      let release;
      const gate = new Promise(resolve => { release = resolve; });
      let inspections = 0;
      await page.route('**/api/v1/exports/quality-check', async route => {
        inspections++;
        if (inspections > 1) return route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 'SYNTHETIC_FAILURE', message: longError }) });
        await gate;
        return route.fulfill({ contentType: 'application/json', body: JSON.stringify({
          rulesVersion: `synthetic-rules-${'long-version-identifier'.repeat(4)}`, checkedFrames: 1, checkedAnnotations: 200, totalFindings: 135, truncated: true,
          frameRevisions: [{ frameId: ids.frame, revision: 7 }],
          findings: Array.from({ length: 100 }, (_, index) => ({ code: ['TINY_BOX', 'LARGE_BOX', 'DUPLICATE_BOX'][index % 3], severity: 'warning', frameId: ids.frame,
            annotationIds: [`synthetic-${index}-${'UnbrokenAnnotationIdentifier'.repeat(6)}`], message: longError })),
        }) });
      });
      const disclosure = page.getByText('导出前质量检查（可选）', { exact: true });
      await expect(page.getByRole('button', { name: '开始质量检查', exact: true })).toBeHidden();
      await disclosure.focus();
      await page.keyboard.press('Enter');
      await page.getByRole('button', { name: '开始质量检查', exact: true }).click();
      await expect(page.getByRole('button', { name: '正在检查…', exact: true })).toBeDisabled();
      await assertResponsiveGeometry(page, 'quality check busy');
      release();
      await expect(page.locator('.quality-results li')).toHaveCount(100);
      await expect(page.locator('.quality-results')).toContainText('检查未完整覆盖');
      await assertResponsiveGeometry(page, '100 long-ID warnings and truncation');
      await assertReachable(page.locator('.quality-results').getByRole('button').last());
      await assertReachable(page.getByTestId('export-zip'));
      await expect(page.getByTestId('export-zip')).toBeEnabled();
      await captureResponsiveScene(page, info, 'quality-report', 'completed_frame_100_synthetic_long_id_warnings_truncated');
      await page.getByRole('button', { name: '开始质量检查', exact: true }).click();
      await expect(page.getByRole('alert').filter({ hasText: '检查失败' })).toContainText('仍可使用上方导出');
      await assertResponsiveGeometry(page, 'quality checker failure fallback');
      await expect(page.getByTestId('export-zip')).toBeEnabled();
      await assertReachable(page.getByTestId('export-zip'));
      expect(inspections).toBe(2);
      expect(calls.writes).toEqual([]);
      expect(calls.unexpected).toEqual([]);
      await captureResponsiveScene(page, info, 'quality-failure', 'quality_check_synthetic_failure_original_export_reachable');
    });
  }

  test('quality report stays open through an unchanged revision poll', async ({ page }) => {
    const calls = await loadSyntheticEditor(page, { review: false, completed: true });
    let inspections = 0;
    await page.route('**/api/v1/exports/quality-check', route => {
      inspections++;
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ rulesVersion: 'synthetic-poll-stability', checkedFrames: 1, checkedAnnotations: 1, totalFindings: 0, truncated: false,
        frameRevisions: [{ frameId: ids.frame, revision: 7 }], findings: [] }) });
    });
    await page.getByText('导出前质量检查（可选）', { exact: true }).click();
    await page.getByRole('button', { name: '开始质量检查', exact: true }).click();
    await expect(page.locator('.quality-results')).toContainText('synthetic-poll-stability');
    const readsBefore = calls.reviewReads;
    await expect.poll(() => calls.reviewReads, { timeout: 8000, intervals: [200] }).toBeGreaterThan(readsBefore);
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    await expect(page.locator('.export-quality')).toHaveAttribute('open', '', { timeout: 1000 });
    await expect(page.locator('.quality-results')).toContainText('synthetic-poll-stability', { timeout: 1000 });
    expect(inspections).toBe(1);
    expect(calls.writes).toEqual([]);
  });

  test('selected upload exposes the full long filename before submission', async ({ page }, info) => {
    await page.setViewportSize({ width: 320, height: 568 });
    const calls = await mockResponsiveApi(page, { empty: true });
    await page.goto('/');
    await page.getByRole('tab', { name: '本地上传', exact: true }).click();
    await page.getByTestId('upload-file').setInputFiles({ name: longName, mimeType: 'video/mp4', buffer: Buffer.from('synthetic UI filename fixture; never uploaded') });
    await expect(page.getByText(longName, { exact: true })).toBeVisible();
    await assertResponsiveGeometry(page, 'selected upload long filename');
    await assertReachable(page.getByTestId('upload-submit'));
    expect(calls.writes).toEqual([]);
    await captureResponsiveScene(page, info, 'upload-filename', 'selected_synthetic_file_before_upload');
  });

  test('P2-UX-001 advanced controls are collapsed and keyboard operable', async ({ page }) => {
    await page.setViewportSize({ width: 360, height: 568 });
    const calls = await loadSyntheticEditor(page);
    const range = page.getByText('指定抽帧范围（可选）', { exact: true });
    await expect(page.getByTestId('start-seconds')).toBeHidden();
    await range.focus();
    await page.keyboard.press('Enter');
    await expect(page.getByTestId('start-seconds')).toBeVisible();
    await assertReachable(page.getByTestId('extract-submit'));
    await page.keyboard.press('Enter');
    await expect(page.getByTestId('start-seconds')).toBeHidden();
    const batch = page.getByText('批量 AI 预标注', { exact: true });
    const summary = page.locator('.review-stats > summary');
    await expect(page.getByRole('button', { name: '选择前 20 张未完成图片' })).toBeHidden();
    await batch.focus();
    await page.keyboard.press('Enter');
    await expect(page.getByRole('button', { name: '选择前 20 张未完成图片' })).toBeVisible();
    await assertResponsiveGeometry(page, 'advanced batch expanded');
    await page.keyboard.press('Enter');
    await expect(page.getByRole('button', { name: '选择前 20 张未完成图片' })).toBeHidden();
    await expect(page.locator('.review-stats')).not.toHaveAttribute('open');
    await summary.focus();
    await page.keyboard.press('Space');
    await expect(page.locator('.review-stats')).toHaveAttribute('open', '');
    await assertResponsiveGeometry(page, 'stats expanded');
    await assertReachable(page.getByTestId('complete-frame'));
    expect(calls.writes).toEqual([]);
  });

  for (const save of ['error', 'conflict']) {
    test(`failed save ${save} survives continuous resizing without implicit submission`, async ({ page }, info) => {
      await page.setViewportSize({ width: 1280, height: 720 });
      const calls = await loadSyntheticEditor(page, { review: false, save });
      const canvas = page.getByTestId('annotation-canvas').locator('.konvajs-content');
      await canvas.scrollIntoViewIfNeeded();
      const box = await canvas.boundingBox();
      await page.mouse.move(box.x + box.width * 0.5, box.y + box.height * 0.45);
      await page.mouse.down();
      await page.mouse.move(box.x + box.width * 0.65, box.y + box.height * 0.65, { steps: 5 });
      await page.mouse.up();
      await expect(page.getByTestId('save-state')).toContainText(save === 'error' ? '保存失败' : '版本冲突');
      expect(calls.writes).toHaveLength(1);
      const readDraft = () => page.evaluate(() => Object.entries(localStorage).filter(([key]) => key.includes(':draft:')));
      const draft = await readDraft();
      expect(draft).toHaveLength(1);
      const savedDraft = JSON.parse(draft[0][1]);
      expect(savedDraft.snapshot.annotations).toHaveLength(2);
      const coordinates = calls.writes[0].annotations;
      for (const [width, height] of [...sizes, [1440, 568], [320, 900], [1920, 568], [1280, 720]]) {
        await page.setViewportSize({ width, height });
        await assertResponsiveGeometry(page, `${save} resize ${width}x${height}`);
        await expect(page.getByTestId('video-select')).toHaveValue(ids.video);
        await expect(page.getByTestId('save-state')).toContainText(save === 'error' ? '保存失败' : '版本冲突');
        expect(await readDraft()).toEqual(draft);
        expect(calls.writes).toHaveLength(1);
        expect(calls.writes[0].annotations).toEqual(coordinates);
        if (save === 'error') await assertReachable(page.getByRole('button', { name: '重试保存', exact: true }));
        else {
          await assertReachable(page.getByTestId('conflict-reload'));
          await assertReachable(page.getByRole('button', { name: '查看最新并保留草稿', exact: true }));
        }
      }
      await page.getByRole('button', { name: '使用指南', exact: true }).click();
      await expect(page.getByRole('navigation', { name: '手册章节' })).toBeVisible();
      await assertResponsiveGeometry(page, `${save} help preserves error`);
      expect(await readDraft()).toEqual(draft);
      expect(calls.writes).toHaveLength(1);
      expect(calls.unexpected).toEqual([]);
      await captureResponsiveScene(page, info, `${save}-draft-preserved`, `manual_draft_${save}_after_continuous_resize`);
    });
  }
});
