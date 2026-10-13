import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { acceptance, externalPath, dedicatedConnection } from '../helpers/postgres.mjs';
import { dedicatedServer } from '../helpers/process.mjs';
import { unusedLoopbackPort } from '../helpers/runtime.mjs';

// Focused observation of the owned public baseline; no DB reset or private import.
await acceptance(async () => {
  const root = externalPath(process.env.P0_API_DATA_DIR, 'Existing dedicated API baseline');
  const output = externalPath(process.env.P01_OFFLINE_OUTPUT, 'P01_OFFLINE_OUTPUT');
  assert.ok(fs.existsSync(path.join(root, 'public-seed-report.json')), 'An actual prepared public baseline is required');
  assert.ok(!fs.existsSync(output), 'Use a fresh external offline evidence directory');
  fs.mkdirSync(output, { recursive: true });
  const token = path.join(root, 'agent-token');
  const agentOrigin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
  const backendOrigin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
  let agent; let backend;
  try {
    agent = await dedicatedServer(process.env.P01_AGENT_BINARY, { MAC_AGENT_ADDR: agentOrigin.replace('http://', ''),
      MAC_MEDIA_ROOT: path.join(root, 'agent-media'), MAC_TOKEN_FILE: token,
      MAC_FFMPEG: process.env.FFMPEG_PATH, MAC_FFPROBE: process.env.FFPROBE_PATH }, agentOrigin, '/health/live');
    await agent.stop();
    assert.ok(agent.child.exitCode !== null || agent.child.signalCode !== null, 'Owned agent stop was not observed');
    backend = await dedicatedServer(process.env.P0_BACKEND_BINARY, {
      APP_DATABASE_URL: dedicatedConnection('P01_ACCEPTANCE_DATABASE_URL_FILE').connectionString,
      APP_DATA_DIR: path.join(root, 'cloud-media'), APP_VERSION: process.env.P0_EXPECTED_SHA,
      APP_FFMPEG: process.env.FFMPEG_PATH, APP_FFPROBE: process.env.FFPROBE_PATH,
      APP_MAC_BRIDGE_URL: agentOrigin, APP_MAC_BRIDGE_TOKEN_FILE: token, APP_ALLOWED_PRIVATE_ORIGINS: agentOrigin,
      APP_SAMPLE_CATALOG_FILE: externalPath(process.env.P01_PUBLIC_SAMPLE_CATALOG, 'Public catalog'),
    }, backendOrigin);
    const cli = fileURLToPath(new URL('../node_modules/@playwright/test/cli.js', import.meta.url));
    const runner = spawn(process.execPath, [cli, 'test', '--project=api', '--grep', 'P01-SAMPLE-002'], {
      env: { ...process.env, P0_BASE_URL: backendOrigin, P01_OFFLINE_PHASE: '1', P01_HAS_PUBLIC_SAMPLES: '1',
        P0_TEST_OUTPUT: path.join(output, 'results'), P0_REPORT_OUTPUT: path.join(output, 'report') }, stdio: 'inherit' });
    const code = await new Promise((resolve, reject) => { runner.once('error', reject); runner.once('exit', resolve); });
    assert.equal(code, 0, 'Public cached closure failed with the actual owned agent stopped');
    fs.writeFileSync(path.join(output, 'public-offline-evidence.json'), JSON.stringify({ observedAt: new Date().toISOString(),
      sourceSHA: process.env.P0_EXPECTED_SHA, sourceKind: 'public_sample', mediaType: 'real_public',
      ownedAgentActuallyStopped: true, backendActuallyRestarted: true, privateFlowVerificationStatus: 'not_run',
      actualMacVerificationStatus: 'not_run', actualCloudVerificationStatus: 'not_run' }, null, 2));
  } finally { if (backend) await backend.stop(); if (agent) await agent.stop(); }
});
