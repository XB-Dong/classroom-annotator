import { test, expect } from '../helpers/p1-session.mjs';
import { upload, extract, snapshot } from '../helpers/api.mjs';
import { dedicatedConnection } from '../helpers/postgres.mjs';
import { waitFor } from '../helpers/process.mjs';
import fs from 'node:fs';
import path from 'node:path';
import pg from 'pg';
import { randomUUID } from 'node:crypto';

test('P1-CAS-001 twenty fresh PostgreSQL barrier races have one winner and no loser side effects', async ({ request, playwright }) => {
  const pool = new pg.Pool({ connectionString: dedicatedConnection('P1_QA_DATABASE_URL_FILE').connectionString, max: 3 });
  const second = await playwright.request.newContext({ baseURL: process.env.P0_BASE_URL });
  const login = await second.post('/api/v1/auth/login', { headers: { Origin: process.env.P0_BASE_URL },
    data: { username: 'p1_account_three', password: 'synthetic-browser-password-2026' } });
  expect(login.status()).toBe(200);
  const session = await login.json();
  const headers = { Origin: process.env.P0_BASE_URL, 'X-CSRF-Token': session.csrfToken };
  const video = await upload(request);
  const { frames } = await extract(request, video.id);
  const frame = frames[0];
  const evidence = [];
  try {
    for (let round = 0; round < 20; round++) {
      const before = await snapshot(request, frame.id);
      const eventsBefore = Number((await pool.query('SELECT count(*) FROM annotation_events WHERE frame_id=$1', [frame.id])).rows[0].count);
      const values = [10, 90].map(x => ({ expectedRevision: before.revision, status: 'in_progress',
        annotations: [{ id: randomUUID(), bbox: [x, 20, 30, 40], source: 'manual' }] }));
      const locker = await pool.connect();
      let pending;
      try {
        await locker.query('BEGIN');
        const pid = Number((await locker.query('SELECT pg_backend_pid() AS pid')).rows[0].pid);
        await locker.query('SELECT id FROM frames WHERE id=$1 FOR UPDATE', [frame.id]);
        pending = [request.put(`/api/v1/frames/${frame.id}/annotations`, { data: values[0] }),
          second.put(`/api/v1/frames/${frame.id}/annotations`, { headers, data: values[1] })];
        for (const promise of pending) void promise.catch(() => {});
        await waitFor(async () => Number((await pool.query(`SELECT count(*) FROM pg_stat_activity WHERE datname=current_database()
          AND state='active' AND query ILIKE '%frames%' AND pid<>$1 AND wait_event_type='Lock'`, [pid])).rows[0].count) >= 2,
        5000, 'Both actual authenticated writers reached the PostgreSQL barrier');
        await locker.query('COMMIT');
      } finally { await locker.query('ROLLBACK'); locker.release(); }
      const responses = await Promise.all(pending);
      expect(responses.map(response => response.status()).sort()).toEqual([200, 409]);
      const winner = responses.findIndex(response => response.status() === 200);
      const committed = await snapshot(request, frame.id);
      expect(committed.revision).toBe(before.revision + 1);
      expect(committed.annotations).toEqual(values[winner].annotations);
      const dbBoxes = (await pool.query('SELECT id,x,y,width,height,source FROM annotations WHERE frame_id=$1', [frame.id])).rows;
      expect(dbBoxes.map(row => ({ id: row.id, bbox: [row.x, row.y, row.width, row.height], source: row.source }))).toEqual(values[winner].annotations);
      const eventRows = (await pool.query('SELECT user_id,revision FROM annotation_events WHERE frame_id=$1 AND revision=$2', [frame.id, committed.revision])).rows;
      expect(eventRows).toHaveLength(1);
      expect(Number((await pool.query('SELECT count(*) FROM annotation_events WHERE frame_id=$1', [frame.id])).rows[0].count)).toBe(eventsBefore + 1);
      const loser = await responses[1 - winner].json();
      expect(loser.code).toBe('REVISION_CONFLICT');
      expect(loser.currentRevision).toBe(committed.revision);
      evidence.push({ round: round + 1, statuses: responses.map(response => response.status()), revision: committed.revision,
        databaseBarrier: true, auditDelta: 1 });
    }
    const previous = await Promise.all(frames.slice(1).map(frame => snapshot(request, frame.id)));
    const different = await Promise.all(frames.slice(1).map((frame, index) => request.put(`/api/v1/frames/${frame.id}/annotations`, {
      data: { expectedRevision: previous[index].revision, status: 'in_progress', annotations: [] } })));
    expect(different.map(response => response.status())).toEqual([200, 200]);
    fs.writeFileSync(path.join(process.env.P1_STACK_EVIDENCE, 'cas-evidence.json'), JSON.stringify({
      sourceSHA: process.env.P0_EXPECTED_SHA, database: 'isolated PostgreSQL17', evidence, differentFrameStatuses: [200, 200] }, null, 2));
  } finally { await second.dispose(); await pool.end(); }
});
