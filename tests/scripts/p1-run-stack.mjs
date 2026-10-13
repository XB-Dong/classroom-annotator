import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import os from 'node:os';
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';
import { acceptance, dedicatedConnection, externalPath, openPostgres, assertEmptyBusiness } from '../helpers/postgres.mjs';
import { command, dedicatedServer } from '../helpers/process.mjs';
import { unusedLoopbackPort } from '../helpers/runtime.mjs';
import { credentials } from '../helpers/p1-session.mjs';
import { sourceFixture } from '../helpers/source-fixture.mjs';
import { aiFixture } from '../helpers/ai-fixture.mjs';
import { postgresProxy } from '../helpers/postgres-proxy.mjs';

await acceptance(async () => {
  const runtime = externalPath(process.env.P1_RUNTIME, 'P1_RUNTIME');
  const root = path.join(runtime, 'p1-stack');
  assert.ok(!fs.existsSync(root), 'Use a fresh P1 stack directory');
  fs.mkdirSync(root, { recursive: true });
  const aiTokenFile = path.join(root, 'ai-token');
  fs.writeFileSync(aiTokenFile, 'synthetic-ci-internal-token-with-enough-entropy', { mode: 0o600 });
  const fixtureVideo = fs.readFileSync(path.join(process.env.P0_FIXTURE_DIR, 'classroom.mp4'));
  const publicFixtures = [
    { bytes: fixtureVideo, sourceFile: 'synthetic-public-1.mp4', durationSeconds: 4 },
    { bytes: fs.readFileSync(path.join(process.env.P0_FIXTURE_DIR, 'classroom20.mp4')), sourceFile: 'synthetic-public-2.mp4', durationSeconds: 20 },
  ];
  assert.notEqual(createHash('sha256').update(publicFixtures[0].bytes).digest('hex'), createHash('sha256').update(publicFixtures[1].bytes).digest('hex'), 'Independent publisher sample identities require distinct generated media');
  for (const item of publicFixtures) fs.writeFileSync(path.join(root, item.sourceFile), item.bytes);
  const catalogFile = path.join(root, 'sample-catalog.json');
  fs.writeFileSync(catalogFile, JSON.stringify({ catalogVersion: '1.0.0', items: [1, 2].map(index => ({
    sampleId: `ci-synthetic-${index}`, version: '1', name: `Synthetic public fixture ${index}`,
    description: 'FFmpeg-generated shapes; no classroom pixels', creator: 'Classroom Annotator CI',
    sourcePageUrl: 'https://github.com/XB-Dong/classroom-annotator', licenseName: 'CC0-1.0',
    licenseUrl: 'https://creativecommons.org/publicdomain/zero/1.0/', sha256: createHash('sha256').update(publicFixtures[index - 1].bytes).digest('hex'),
    sizeBytes: publicFixtures[index - 1].bytes.length, width: 320, height: 240, durationSeconds: publicFixtures[index - 1].durationSeconds,
    retrievedAt: new Date().toISOString(), sourceFile: publicFixtures[index - 1].sourceFile })) }));
  const backendOrigin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
  const frontendOrigin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
  const pythonOrigin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
  const followerOrigin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
  const fixture = await sourceFixture(process.env.P0_FIXTURE_DIR, 'synthetic-source', 'synthetic-dataset');
  const pool = await openPostgres('P01_ACCEPTANCE_DATABASE_URL_FILE');
  const databaseProxy = await postgresProxy(dedicatedConnection('P01_ACCEPTANCE_DATABASE_URL_FILE').connectionString);
  const env = { APP_DATABASE_URL: dedicatedConnection('P01_ACCEPTANCE_DATABASE_URL_FILE').connectionString,
    APP_DATA_DIR: path.join(root, 'media'), APP_VERSION: process.env.P0_EXPECTED_SHA,
    APP_FFMPEG: process.env.FFMPEG_PATH, APP_FFPROBE: process.env.FFPROBE_PATH,
    APP_AUTH_INVITE_CODE: 'synthetic-ci-invite', APP_AUTH_COOKIE_SECURE: 'false',
    APP_AUTH_TRUSTED_ORIGINS: `${frontendOrigin},${backendOrigin},${followerOrigin}`,
    APP_DATASET_API_URL: fixture.apiOrigin, APP_DATASET_ID: '42', APP_DATASET_API_KEY: 'synthetic-dataset',
    APP_ALLOWED_PRIVATE_ORIGINS: `${fixture.apiOrigin},${fixture.mediaOrigin}`,
    APP_ALLOWED_MEDIA_ORIGINS: fixture.mediaOrigin,
    APP_LOCAL_SOURCE_DIRS: process.env.P0_FIXTURE_DIR,
    APP_SAMPLE_CATALOG_FILE: catalogFile,
    APP_AI_URL: pythonOrigin, APP_AI_TOKEN_FILE: aiTokenFile };
  const dist = fileURLToPath(new URL('../../frontend/dist/', import.meta.url));
  const proxy = http.createServer((request, response) => {
    if (/^\/(?:api|media|health)\//.test(request.url)) {
      const upstream = http.request(new URL(request.url, backendOrigin), { method: request.method, headers: request.headers }, incoming => {
        response.writeHead(incoming.statusCode, incoming.headers); incoming.pipe(response);
      });
      upstream.on('error', () => { if (!response.headersSent) response.writeHead(502); response.end(); });
      response.on('close', () => upstream.destroy()); request.pipe(upstream); return;
    }
    const route = decodeURIComponent(new URL(request.url, frontendOrigin).pathname);
    let file = path.resolve(dist, `.${route}`);
    const relative = path.relative(dist, file);
    if (relative.startsWith('..') || path.isAbsolute(relative)) { response.writeHead(400); response.end(); return; }
    if (!fs.existsSync(file) || fs.statSync(file).isDirectory()) file = path.join(dist, 'index.html');
    response.setHeader('Content-Type', ({ '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml' })[path.extname(file)] || 'application/octet-stream');
    fs.createReadStream(file).pipe(response);
  });
  let backend, python, follower, syntheticAI, resourceTimer;
  const backendEnvironment = { ...env, APP_DATABASE_URL: databaseProxy.connectionString };
  const startPython = () => dedicatedServer(process.env.P1_PYTHON_BINARY, {
    PYTHONPATH: fileURLToPath(new URL('../../ai-service/src/', import.meta.url)), AI_MODEL_PATH: process.env.P1_MODEL_PATH,
    AI_INTERNAL_TOKEN: fs.readFileSync(aiTokenFile, 'utf8'), PYTHONSTARTUP: '' }, pythonOrigin, '/health/ready',
    ['-m', 'uvicorn', 'classroom_ai.app:create_app', '--factory', '--host', '127.0.0.1',
      '--port', new URL(pythonOrigin).port, '--workers', '1', '--no-access-log']);
  const resources = { samples: 0, goPeakRssKiB: 0, followerPeakRssKiB: 0, pythonPeakRssKiB: 0, combinedPeakRssKiB: 0,
    measuredProcesses: ['primary-go', 'test-follower-go', 'owned-python'],
    machine: { platform: os.platform(), arch: os.arch(), logicalCPUs: os.cpus().length,
      totalMemoryBytes: os.totalmem(), cpuModel: os.cpus()[0]?.model } };
  try {
    await command(process.env.P01_MIGRATE_BINARY, ['up'], env);
    await assertEmptyBusiness(pool);
    await command(process.env.P01_SEED_BINARY, ['--catalog', catalogFile, '--report', path.join(root, 'sample-seed.json')], env);
    python = await startPython();
    backend = await dedicatedServer(process.env.P0_BACKEND_BINARY, backendEnvironment, backendOrigin);
    const sampleResources = () => {
      if (os.platform() !== 'linux') return;
      const rss = pid => {
        try { return Number(fs.readFileSync(`/proc/${pid}/status`, 'utf8').match(/^VmRSS:\s+(\d+) kB$/m)?.[1] || 0); }
        catch { return 0; }
      };
      const goRSS = rss(backend.child.pid), pythonRSS = rss(python.child.pid);
      const followerRSS = follower ? rss(follower.child.pid) : 0;
      resources.samples++;
      resources.goPeakRssKiB = Math.max(resources.goPeakRssKiB, goRSS);
      resources.followerPeakRssKiB = Math.max(resources.followerPeakRssKiB, followerRSS);
      resources.pythonPeakRssKiB = Math.max(resources.pythonPeakRssKiB, pythonRSS);
      resources.combinedPeakRssKiB = Math.max(resources.combinedPeakRssKiB, goRSS + pythonRSS + followerRSS);
    };
    sampleResources(); resourceTimer = setInterval(sampleResources, 1000); resourceTimer.unref();
    await new Promise(resolve => proxy.listen(Number(new URL(frontendOrigin).port), '127.0.0.1', resolve));
    for (const username of [credentials.username, 'p1_account_two', 'p1_account_three']) {
      const registered = await fetch(`${frontendOrigin}/api/v1/auth/register`, { method: 'POST',
        headers: { Origin: frontendOrigin, 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...credentials, username, inviteCode: env.APP_AUTH_INVITE_CODE }) });
      assert.equal(registered.status, 201, 'Fresh synthetic account registration failed');
    }
    const cli = fileURLToPath(new URL('../node_modules/@playwright/test/cli.js', import.meta.url));
    const args = ['test', 'integration/api.spec.mjs', 'integration/auth.spec.mjs', 'integration/collaboration.spec.mjs',
      'integration/help.spec.mjs', 'integration/ai-review.spec.mjs', 'integration/p1-sources.spec.mjs',
      'integration/public-samples.spec.mjs', 'integration/extraction-race.spec.mjs', '--project=api'];
    const run = async (arguments_, additionalEnvironment = {}) => {
      const child = spawn(process.execPath, [cli, ...arguments_], { cwd: fileURLToPath(new URL('../', import.meta.url)),
        env: { ...process.env, P0_BASE_URL: frontendOrigin,
        P1_STACK_EVIDENCE: root, P1_QA_DATABASE_URL_FILE: process.env.P01_ACCEPTANCE_DATABASE_URL_FILE,
        P01_HAS_PUBLIC_SAMPLES: '1', ...additionalEnvironment }, stdio: 'inherit' });
      const code = await new Promise((resolve, reject) => { child.once('error', reject); child.once('exit', resolve); });
      assert.equal(code, 0, 'P1 PostgreSQL/auth/FFmpeg/browser acceptance failed');
    };
    await run(args);
    await python.stop();
    assert.ok(python.child.signalCode || python.child.exitCode !== null, 'Actual Python must stop before fixture phase');
    const fixturePhase = { dataType: 'fixture_not_real_model', sourceSHA: process.env.P0_EXPECTED_SHA,
      startedAt: new Date().toISOString(), actualPythonStopped: true, restoredActualPython: false, status: 'running' };
    fs.writeFileSync(path.join(root, 'fixture-review-evidence.json'), JSON.stringify(fixturePhase, null, 2));
    syntheticAI = await aiFixture(pythonOrigin, fs.readFileSync(aiTokenFile, 'utf8'), async action => {
      if (action === 'start-follower') {
        assert.ok(!follower, 'Follower already started');
        follower = await dedicatedServer(process.env.P0_BACKEND_BINARY, env, followerOrigin);
      } else if (action === 'stop-follower') {
        if (follower) await follower.stop(); follower = undefined;
      } else if (action === 'disconnect-primary-db') databaseProxy.disconnect();
      else if (action === 'reconnect-primary-db') databaseProxy.reconnect();
      else if (action === 'restart-primary') {
        await backend.stop(); backend = await dedicatedServer(process.env.P0_BACKEND_BINARY, backendEnvironment, backendOrigin);
      } else throw new Error('Unknown test-owned action');
    });
    try {
      await run(['test', 'integration/review-fixture.spec.mjs', 'integration/review-batch-fixture.spec.mjs',
        'integration/review-publication-fixture.spec.mjs', 'integration/scheduler-fixture.spec.mjs', '--project=api'], {
        P1_AI_FIXTURE_PHASE: 'fixture_not_real_model', P1_AI_FIXTURE_ORIGIN: pythonOrigin,
        P1_AI_FIXTURE_CONTROL_TOKEN: syntheticAI.controlToken, P1_FOLLOWER_ORIGIN: followerOrigin,
        P0_TEST_OUTPUT: path.join(root, 'fixture-api-results'), P0_REPORT_OUTPUT: path.join(root, 'fixture-api-report') });
      fixturePhase.status = 'passed';
    } catch (error) { fixturePhase.status = 'failed'; throw error; }
    finally {
      fixturePhase.requests = syntheticAI.evidence;
      try {
        await syntheticAI.stop(); syntheticAI = undefined;
        if (follower) await follower.stop(); follower = undefined;
        databaseProxy.reconnect();
        python = await startPython(); fixturePhase.restoredActualPython = true;
      } finally {
        fixturePhase.finishedAt = new Date().toISOString();
        fs.writeFileSync(path.join(root, 'fixture-review-evidence.json'), JSON.stringify(fixturePhase, null, 2));
      }
    }
    await command(process.env.P01_SEED_BINARY, ['--catalog', catalogFile, '--report', path.join(root, 'sample-reseed.json')], env);
    await backend.stop();
    backend = await dedicatedServer(process.env.P0_BACKEND_BINARY, backendEnvironment, backendOrigin);
    await run(['test', 'integration/public-samples.spec.mjs', '--project=api', '--grep', 'P01-SAMPLE-002'], { P01_OFFLINE_PHASE: '1' });
    if (process.argv.includes('--e2e')) {
      // Preserve the real IP authentication limiter between independent suites.
      await delay(61_000);
      await run(['test', 'e2e/workflow.spec.mjs', 'e2e/public-samples.spec.mjs', 'e2e/p1-workflow.spec.mjs', 'e2e/p1-conflict.spec.mjs', '--project=chromium']);
    }
    await python.stop();
    assert.ok(python.child.signalCode || python.child.exitCode !== null, 'The owned Python service did not stop');
    fs.writeFileSync(path.join(root, 'ai-stopped-evidence.json'), JSON.stringify({ sourceSHA: process.env.P0_EXPECTED_SHA,
      observedAt: new Date().toISOString(), ownedPythonStopped: true, pid: python.child.pid }, null, 2));
    await run(['test', 'integration/degrade.spec.mjs', '--project=api'], { P1_AI_STOPPED: '1' });
    if (process.argv.includes('--e2e')) await run(['test', 'e2e/workflow.spec.mjs', '--project=chromium', '--grep', 'P0-E2E-001'], {
      P1_AI_STOPPED: '1', P0_TEST_OUTPUT: path.join(root, 'after-ai-stop-browser-results'),
      P0_REPORT_OUTPUT: path.join(root, 'after-ai-stop-browser-report') });
    fixture.assertClean();
    const routing = (await pool.query('SELECT source_kind,storage_provider,count(*) FROM videos GROUP BY source_kind,storage_provider')).rows;
    assert.ok(routing.some(row => row.source_kind === 'upload'));
    assert.ok(routing.every(row => row.storage_provider === 'local'), 'New resources must use current-host media');
    fs.writeFileSync(path.join(root, 'stack-evidence.json'), JSON.stringify({ sourceSHA: process.env.P0_EXPECTED_SHA,
      observedAt: new Date().toISOString(), dataType: 'synthetic', nativeAgent: false, tunnel: false,
      routing, fixtureRequests: fixture.evidence, resources, browserRun: process.argv.includes('--e2e') }, null, 2));
  } finally {
    clearInterval(resourceTimer);
    fs.writeFileSync(path.join(root, 'resource-evidence.json'), JSON.stringify(resources, null, 2));
    proxy.closeAllConnections(); if (proxy.listening) await new Promise(resolve => proxy.close(resolve));
    if (backend) await backend.stop(); if (follower) await follower.stop();
    if (syntheticAI) await syntheticAI.stop(); if (python) await python.stop();
    await databaseProxy.stop(); await fixture.stop(); await pool.end();
  }
});
