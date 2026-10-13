import { test, expect } from '../helpers/p1-session.mjs';
import fs from 'node:fs';
import { createHash } from 'node:crypto';

test('P1-HELP-001 anonymous manual is the full canonical document and unavailable QA preserves it', async ({ request, playwright }) => {
  const anonymous = await playwright.request.newContext({ baseURL: process.env.P0_BASE_URL });
  try {
    const response = await anonymous.get('/api/v1/help/document');
    expect(response.status()).toBe(200);
    const document = await response.json();
    const canonical = fs.readFileSync(new URL('../../docs/product/ANNOTATOR-GUIDE.zh.md', import.meta.url), 'utf8');
    expect(document.content).toBe(canonical);
    expect(document.documentSHA256).toBe(createHash('sha256').update(canonical).digest('hex'));
    expect(document.sections.length).toBeGreaterThan(5);
    expect(new Set(document.sections.map(section => section.id)).size).toBe(document.sections.length);
    for (const section of document.sections) expect(section.anchor).toBe(`#${section.id}`);
    expect((await anonymous.post('/api/v1/help/questions', { data: { question: '如何开始预标注？' } })).status()).toBe(401);
    for (const question of ['', '问'.repeat(1001)])
      expect((await request.post('/api/v1/help/questions', { data: { question } })).status()).toBe(422);
    const unavailable = await request.post('/api/v1/help/questions', { data: { question: '如何处理版本冲突？' } });
    expect(unavailable.status()).toBe(503);
    expect((await unavailable.json()).code).toBe('HELP_UNAVAILABLE');
    expect((await request.get('/api/v1/help/document')).status()).toBe(200);
  } finally { await anonymous.dispose(); }
});
