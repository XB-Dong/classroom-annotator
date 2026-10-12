import assert from 'node:assert/strict';

const app = process.env.P0_BASE_URL || 'http://127.0.0.1:8180';
const prometheus = process.env.P0_PROMETHEUS_URL || 'http://127.0.0.1:9090';
const grafana = process.env.P0_GRAFANA_URL || 'http://127.0.0.1:3000';
const expectedSHA = process.env.P0_EXPECTED_SHA;
assert.ok(expectedSHA, 'P0_EXPECTED_SHA is required');
const grafanaHeaders = {};
if (process.env.P0_GRAFANA_TOKEN) grafanaHeaders.Authorization = `Bearer ${process.env.P0_GRAFANA_TOKEN}`;
else {
  assert.ok(process.env.P0_GRAFANA_PASSWORD, 'Set external P0_GRAFANA_TOKEN or P0_GRAFANA_PASSWORD');
  grafanaHeaders.Authorization = `Basic ${Buffer.from(`${process.env.P0_GRAFANA_USER || 'admin'}:${process.env.P0_GRAFANA_PASSWORD}`).toString('base64')}`;
}
async function json(origin, route, headers = {}) {
  const response = await fetch(`${origin}${route}`, { headers, signal: AbortSignal.timeout(10_000) });
  assert.equal(response.status, 200, `${route} HTTP status`);
  return response.json();
}
const health = await json(app, '/health/ready');
assert.equal(health.status, 'ok');
assert.equal(health.version, expectedSHA);
const targets = await json(prometheus, '/api/v1/targets');
assert.equal(targets.status, 'success');
for (const job of ['backend', 'prometheus', 'node']) {
  const selected = targets.data.activeTargets.filter(target => target.labels.job === job);
  assert.equal(selected.length, 1, `expected exactly one ${job} target`);
  assert.equal(selected[0].health, 'up', `${job} scrape health`);
  assert.equal(selected[0].lastError, '', `${job} scrape error`);
}
for (const [query, minimum] of [
  ['sum(classroom_http_requests_total)', 1],
  ['sum(classroom_jobs_total)', 1],
  ['classroom_job_queue_depth', 0],
  ['node_memory_MemAvailable_bytes', 1],
]) {
  const result = await json(prometheus, `/api/v1/query?query=${encodeURIComponent(query)}`);
  assert.equal(result.status, 'success');
  assert.ok(result.data.result.length > 0, `${query} has observed samples`);
  for (const item of result.data.result) assert.ok(Number.isFinite(Number(item.value[1])) && Number(item.value[1]) >= minimum, `${query} sample`);
}
const datasource = await json(grafana, '/api/datasources/uid/p0-prometheus', grafanaHeaders);
assert.equal(datasource.type, 'prometheus');
assert.equal(datasource.url, 'http://prometheus:9090');
assert.equal(datasource.readOnly, true);
const dashboard = await json(grafana, '/api/dashboards/uid/classroom-p0', grafanaHeaders);
assert.equal(dashboard.dashboard.editable, false);
assert.equal(dashboard.meta.provisioned, true);
assert.ok(dashboard.dashboard.panels.length >= 8);
console.log('P0-OPS/OBS: deployed SHA, three healthy scrape targets, observed app/job/host samples and provisioned read-only Grafana passed.');
