export const longName = `合成课堂可读性验证_${'LongUnbrokenVideoIdentifier'.repeat(5)}_${'长文件名测试'.repeat(10)}.mp4`;
export const longError = `合成服务失败：${'RETRY_WITH_A_VERY_LONG_REQUEST_IDENTIFIER_'.repeat(5)}。请保留草稿，检查连接后重试。`;
export const ids = { video: 'synthetic-video', batch: 'synthetic-batch', frame: 'synthetic-frame' };
export const originalAnnotation = { id: 'manual-head', bbox: [48, 48, 48, 48], source: 'manual' };
const stamp = '2026-10-14T12:00:00Z';
const syntheticImage = '<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360"><rect width="640" height="360" fill="#e9edf1"/><path d="M0 120h640M0 240h640M160 0v360M320 0v360M480 0v360" stroke="#c4ccd5"/><circle cx="72" cy="72" r="20" fill="#8aa8ba"/><text x="170" y="180" fill="#35495b" font-size="24">SYNTHETIC UI FIXTURE</text></svg>';

// This fixture exercises rendered frontend behavior only. It never contacts Go, PG, media or an LLM.
export async function mockResponsiveApi(page, { authenticated = true, empty = false, review = true, serviceError = false, completed = false, save = 'ok' } = {}) {
  const calls = { writes: [], unexpected: [], questions: 0, reviewReads: 0 };
  let snapshot = { frameId: ids.frame, revision: 7, status: completed ? 'completed' : 'in_progress', annotations: [structuredClone(originalAnnotation)] };
  const user = { id: 'synthetic-user', username: 'synthetic_user', displayName: `合成标注员_${'UnbrokenAccountName'.repeat(5)}`, role: 'annotator', workspaceId: 'synthetic-workspace' };
  const session = { user, csrfToken: 'synthetic-ui-only' };
  const video = { id: ids.video, name: longName, width: 640, height: 360, durationSeconds: 12, status: 'ready', sourceKind: 'upload', storageProvider: 'local', createdAt: stamp };
  const frame = { id: ids.frame, videoId: ids.video, batchId: ids.batch, timestampMs: 1000, width: 640, height: 360, imageUrl: '/media/p2-synthetic.svg', status: snapshot.status, revision: snapshot.revision };
  const reviewSnapshot = () => ({ ...snapshot, candidates: review ? [{ id: 'synthetic-candidate', runId: 'synthetic-run', bbox: [220, 100, 24, 24], confidence: 0.42, reviewState: 'pending', riskReasons: ['LOW_CONFIDENCE', 'SMALL_HEAD'] }] : [], issues: [], currentRun: review ? { id: 'synthetic-run', status: 'completed', modelId: `synthetic-model-${'longIdentifier'.repeat(8)}`, modelSHA256: 'a'.repeat(64), pipelineVersion: 'synthetic-ui-only', baseRevision: snapshot.revision } : null });
  await page.route('**/media/p2-synthetic.svg', route => route.fulfill({ contentType: 'image/svg+xml', body: syntheticImage }));
  await page.route('**/api/v1/**', async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace('/api/v1', '');
    const method = request.method();
    const fulfill = (body, status = 200) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
    const fail = (status, code = 'SYNTHETIC_FAILURE') => fulfill({ code, message: longError }, status);
    if (path === '/auth/me') return authenticated ? fulfill(session) : fulfill({ code: 'UNAUTHENTICATED' }, 401);
    if (path === '/auth/login') return fail(503);
    if (path === '/auth/register') return fail(503);
    if (path === '/auth/logout') return route.fulfill({ status: 204 });
    if (path === '/videos') return serviceError ? fail(503) : fulfill({ videos: empty ? [] : [video] });
    if (path === '/jobs') return fulfill({ jobs: serviceError ? [] : [
      { id: 'synthetic-running', kind: 'extraction', videoId: ids.video, status: 'running', progress: 0.42, createdAt: stamp },
      { id: 'synthetic-failed', kind: 'import', status: 'failed', progress: 0, error: longError, createdAt: stamp },
    ] });
    if (path === '/media-storage/status') return fulfill({ providers: [{ provider: 'local', available: true, reasonCode: null }], sources: ['upload', 'public_sample', 'dataset', 'local_directory'].map(sourceKind => ({ sourceKind, available: true, reasonCode: null })) });
    if (path === '/samples') return serviceError ? fail(503) : fulfill({ items: [] });
    if (path === `/videos/${ids.video}/batches`) return fulfill({ batches: [{ id: ids.batch, videoId: ids.video, jobId: 'synthetic-extraction', createdAt: stamp, frameCount: 1 }] });
    if (path === `/videos/${ids.video}/frames`) return fulfill({ frames: [frame] });
    if (path === `/videos/${ids.video}/progress`) return fulfill({ total: 1, notStarted: 0, inProgress: snapshot.status === 'completed' ? 0 : 1, completed: snapshot.status === 'completed' ? 1 : 0 });
    if (path === `/videos/${ids.video}/review-stats`) return fulfill({ validImages: 1, suspiciousImages: 1, failedImages: 0, candidates: review ? 1 : 0, pending: review ? 1 : 0, accepted: 0, adjusted: 0, rejected: 0, riskyCandidates: review ? 1 : 0, unresolvedRiskCandidates: review ? 1 : 0, unresolvedRiskImages: 0, unresolvedRiskFrameIds: review ? [ids.frame] : [], manualAdded: 1, rates: { acceptedUnchanged: null, adjusted: null, rejected: null, suspiciousCandidates: 1, suspiciousImages: 1, reviewed: 0 } });
    if (path === `/frames/${ids.frame}/review`) { calls.reviewReads++; return fulfill(reviewSnapshot()); }
    if (path === `/frames/${ids.frame}/annotations` && method === 'PUT') {
      calls.writes.push(request.postDataJSON());
      if (save !== 'ok') return fail(save === 'conflict' ? 409 : 503, save === 'conflict' ? 'REVISION_CONFLICT' : 'SYNTHETIC_FAILURE');
      const update = request.postDataJSON();
      snapshot = { ...snapshot, ...update, revision: snapshot.revision + 1 };
      delete snapshot.expectedRevision;
      return fulfill(snapshot);
    }
    if (path === '/help/document') return fulfill({ documentVersion: 'synthetic-p2', documentSHA256: 'b'.repeat(64), content: `# 合成使用指南\n## 保存与冲突 {#save}\n${longError}\nhttps://example.invalid/${'long-reference-without-breaks'.repeat(8)}\n## 导出复核 {#export}\n只导出已完成人工复核的帧。`, sections: [{ id: 'save', title: '保存与冲突', anchor: '#save' }, { id: 'export', title: '导出复核', anchor: '#export' }] });
    if (path === '/help/questions') { calls.questions++; return fail(503); }
    calls.unexpected.push(`${method} ${path}`);
    return fail(501, 'UNEXPECTED_SYNTHETIC_ROUTE');
  });
  return calls;
}

export async function loadSyntheticEditor(page, options) {
  const calls = await mockResponsiveApi(page, options);
  await page.goto('/');
  await page.getByTestId('video-select').selectOption(ids.video);
  await page.getByTestId('annotation-canvas').locator('canvas').first().waitFor({ state: 'visible' });
  await page.getByTestId('save-state').filter({ hasText: '已保存' }).waitFor();
  return calls;
}
