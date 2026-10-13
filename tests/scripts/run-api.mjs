import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import { randomUUID } from 'node:crypto';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { acceptance, externalPath, dedicatedConnection, openPostgres, assertEmptyBusiness } from '../helpers/postgres.mjs';
import { command, dedicatedServer } from '../helpers/process.mjs';
import { unusedLoopbackPort } from '../helpers/runtime.mjs';
import { sourceFixture } from '../helpers/source-fixture.mjs';

const frontend = fileURLToPath(new URL('../../frontend/dist/', import.meta.url));

// Real HTTP streaming proxy + built frontend for same-origin browser acceptance.
async function frontendProxy(backend) {
  const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml' };
  const server = http.createServer((request, response) => {
    if (/^\/(?:api|media|health)\//.test(request.url)) {
      const upstream = http.request(new URL(request.url, backend), { method: request.method, headers: request.headers }, incoming => {
        response.writeHead(incoming.statusCode, incoming.headers); incoming.pipe(response);
      });
      upstream.on('error', () => { if (!response.headersSent) response.writeHead(502); response.end(); });
      response.on('close', () => upstream.destroy()); request.pipe(upstream); return;
    }
    const route = decodeURIComponent(new URL(request.url, 'http://fixture').pathname);
    let file = path.resolve(frontend, `.${route}`);
    const relative = path.relative(frontend, file);
    if (relative.startsWith(`..${path.sep}`) || path.isAbsolute(relative)) { response.writeHead(400); response.end(); return; }
    if (!fs.existsSync(file) || fs.statSync(file).isDirectory()) file = path.join(frontend, 'index.html');
    response.setHeader('Content-Type', types[path.extname(file)] || 'application/octet-stream');
    fs.createReadStream(file).pipe(response);
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  return { origin: `http://127.0.0.1:${server.address().port}`, stop: async () => {
    server.closeAllConnections(); await new Promise(resolve => server.close(resolve));
  } };
}

await acceptance(async () => {
  const root = externalPath(process.env.P0_API_DATA_DIR, 'P0_API_DATA_DIR');
  assert.ok(!fs.existsSync(root), 'Use a fresh dedicated API media directory');
  fs.mkdirSync(root, { recursive: true });
  const version = process.env.P0_EXPECTED_SHA;
  assert.match(version || '', /^[0-9a-f]{40}$/, 'Set exact binary source SHA');
  const token = randomUUID(); const sourceToken = randomUUID(); const datasetKey = randomUUID();
  const secrets = Object.fromEntries(Object.entries({ agent: token, source: sourceToken, dataset: datasetKey }).map(([name, value]) => {
    const file = path.join(root, `${name}-token`); fs.writeFileSync(file, value, { mode: 0o600 }); return [name, file];
  }));
  const fixture = await sourceFixture(process.env.P0_FIXTURE_DIR, sourceToken, datasetKey);
  const agentOrigin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
  const backendOrigin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
  const pool = await openPostgres('P01_ACCEPTANCE_DATABASE_URL_FILE');
  const environment = { APP_DATABASE_URL: dedicatedConnection('P01_ACCEPTANCE_DATABASE_URL_FILE').connectionString,
    APP_DATA_DIR: path.join(root, 'cloud-media'), APP_VERSION: version,
    APP_FFMPEG: process.env.FFMPEG_PATH || 'ffmpeg', APP_FFPROBE: process.env.FFPROBE_PATH || 'ffprobe',
    APP_MAC_BRIDGE_URL: agentOrigin, APP_MAC_BRIDGE_TOKEN_FILE: secrets.agent,
    APP_ALLOWED_PRIVATE_ORIGINS: agentOrigin };
  let agent; let backend; let frontendServer;
  try {
    await command(process.env.P01_MIGRATE_BINARY, ['up'], environment);
    await assertEmptyBusiness(pool);
    const schemaVersion = Number((await pool.query('SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1')).rows[0].version_id);
    assert.equal(schemaVersion, 4, 'Full-route acceptance requires schema version 4');
    agent = await dedicatedServer(process.env.P01_AGENT_BINARY, { MAC_AGENT_ADDR: agentOrigin.replace('http://', ''),
      MAC_MEDIA_ROOT: path.join(root, 'agent-media'), MAC_TOKEN_FILE: secrets.agent,
      MAC_SOURCE_SERVICE_URL: fixture.origin, MAC_SOURCE_TOKEN_FILE: secrets.source,
      MAC_FFMPEG: environment.APP_FFMPEG, MAC_FFPROBE: environment.APP_FFPROBE,
      MAC_DATASET_API_URL: fixture.apiOrigin, MAC_DATASET_ID: '42', MAC_DATASET_API_KEY_FILE: secrets.dataset,
      MAC_DATASET_ALLOWED_PRIVATE_ORIGINS: `${fixture.apiOrigin},${fixture.mediaOrigin}`,
      MAC_DATASET_ALLOWED_MEDIA_ORIGINS: fixture.mediaOrigin, MAC_AGENT_VERSION: version, APP_VERSION: version }, agentOrigin, '/health/live');
    if (process.env.P01_PUBLIC_SAMPLE_CATALOG) {
      await command(process.env.P01_SEED_BINARY, ['--catalog', externalPath(process.env.P01_PUBLIC_SAMPLE_CATALOG, 'Public sample catalog')], environment);
    }
    backend = await dedicatedServer(process.env.P0_BACKEND_BINARY, environment, backendOrigin);
    frontendServer = await frontendProxy(backendOrigin);
    const cli = fileURLToPath(new URL('../node_modules/@playwright/test/cli.js', import.meta.url));
    const args = ['test', '--project=api'];
    if (process.argv.includes('--e2e')) args.push('--project=chromium');
    const runner = spawn(process.execPath, [cli, ...args], { env: { ...process.env, P0_BASE_URL: frontendServer.origin,
      P01_LOCAL_STACK: '1', P01_HAS_PUBLIC_SAMPLES: process.env.P01_PUBLIC_SAMPLE_CATALOG ? '1' : '0' }, stdio: 'inherit' });
    const code = await new Promise((resolve, reject) => { runner.once('error', reject); runner.once('exit', resolve); });
    assert.equal(code, 0, 'Real PostgreSQL/native agent API/browser acceptance failed');
    fixture.assertClean();
    fs.writeFileSync(path.join(root, 'stack-evidence.json'), JSON.stringify({ observedAt: new Date().toISOString(), sourceSHA: version,
      dataType: 'synthetic', PostgreSQL: '17', schemaVersion: 4, nativeAgent: true,
      actualMacVerificationStatus: 'not_run', browserRun: process.argv.includes('--e2e'), fixtureRequests: fixture.evidence }, null, 2));
  } finally {
    if (frontendServer) await frontendServer.stop(); if (backend) await backend.stop(); if (agent) await agent.stop();
    await fixture.stop(); await pool.end();
  }
});
