import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createHash, randomUUID } from 'node:crypto';
import { acceptance, externalPath, dedicatedConnection, openPostgres, businessRows, mediaReferenceRows } from '../helpers/postgres.mjs';
import { dedicatedServer, command } from '../helpers/process.mjs';
import { unusedLoopbackPort } from '../helpers/runtime.mjs';
import { api } from '../helpers/http.mjs';
import { bridgeFixture } from '../helpers/bridge-fixture.mjs';
import { cancelPublishRace } from '../helpers/job-race.mjs';

await acceptance(async () => {
  assert.match(process.env.P0_EXPECTED_SHA || '', /^[0-9a-f]{40}$/);
  const root = externalPath(process.env.P01_JOB_OUTPUT, 'P01_JOB_OUTPUT');
  assert.ok(!fs.existsSync(root), 'Use a fresh dedicated external job evidence directory');
  fs.mkdirSync(root, { recursive: true });
  const bytes = fs.readFileSync(externalPath(path.join(process.env.P0_FIXTURE_DIR || '', 'source-image.jpg'), 'Synthetic JPEG'));
  const token = randomUUID(); const tokenFile = path.join(root, 'bridge-token');
  fs.writeFileSync(tokenFile, token, { mode: 0o600 });
  const bridge = await bridgeFixture(token, bytes, process.env.P0_EXPECTED_SHA);
  const connection = dedicatedConnection();
  const pool = await openPostgres();
  const origin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
  let server;
  try {
    const environment = { APP_DATABASE_URL: connection.connectionString, APP_DATA_DIR: root,
      APP_VERSION: process.env.P0_EXPECTED_SHA, APP_FFMPEG: process.env.FFMPEG_PATH,
      APP_FFPROBE: process.env.FFPROBE_PATH, APP_MAX_FRAMES: '1000',
      APP_MAC_BRIDGE_URL: bridge.origin, APP_MAC_BRIDGE_TOKEN_FILE: tokenFile,
      APP_ALLOWED_PRIVATE_ORIGINS: bridge.origin };
    await command(process.env.P01_MIGRATE_BINARY, ['up'], environment);
    const videoId = randomUUID();
    await pool.query(`INSERT INTO videos(id,name,width,height,duration_seconds,status,created_at,storage_key,source_kind,storage_provider,media_sha256,media_size_bytes)
      VALUES($1,'Synthetic publication race',320,240,20,'ready',$2,$3,'mac','mac',$4,$5)`,
    [videoId, new Date().toISOString(), randomUUID(), createHash('sha256').update(bytes).digest('hex'), bytes.length]);
    server = await dedicatedServer(process.env.P0_BACKEND_BINARY, environment, origin);
    const evidence = { sourceSHA: process.env.P0_EXPECTED_SHA, observedAt: new Date().toISOString(), dataType: 'synthetic',
      ...await cancelPublishRace(api(origin), pool, bridge, videoId), bridgeEvents: bridge.events };
    if (process.env.P0_RESTORE_BASELINE) {
      const file = externalPath(process.env.P0_RESTORE_BASELINE, 'P0_RESTORE_BASELINE');
      const baseline = JSON.parse(fs.readFileSync(file, 'utf8'));
      assert.equal(baseline.database, connection.database);
      baseline.businessRows = await businessRows(pool);
      baseline.mediaReferenceRows = await mediaReferenceRows(pool);
      fs.writeFileSync(file, JSON.stringify(baseline, null, 2));
    }
    fs.writeFileSync(path.join(root, 'job-race-evidence.json'), JSON.stringify(evidence, null, 2));
    console.log('P01-DB-005 passed: observed cancellation-first and publication-first PostgreSQL locking produce consistent terminal jobs/batches/frames. Synthetic bridge evidence does not claim actual Mac execution.');
  } finally { if (server) await server.stop(); await bridge.stop(); await pool.end(); }
});
