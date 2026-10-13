import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import pg from 'pg';

export const repository = path.resolve(fileURLToPath(new URL('../..', import.meta.url)));

export function externalPath(value, label) {
  assert.ok(value && path.isAbsolute(value), `${label} must be an absolute external path`);
  const resolved = path.resolve(value);
  const relative = path.relative(repository, resolved);
  assert.ok(relative.startsWith(`..${path.sep}`) || path.isAbsolute(relative), `${label} must be outside repository`);
  return resolved;
}

export function dedicatedConnection(fileVariable = 'P01_DATABASE_URL_FILE') {
  const file = externalPath(process.env[fileVariable], fileVariable);
  const connectionString = fs.readFileSync(file, 'utf8').trim();
  let parsed;
  try { parsed = new URL(connectionString); } catch { throw new Error('Invalid dedicated PostgreSQL connection file'); }
  assert.ok(['postgres:', 'postgresql:'].includes(parsed.protocol), 'A real PostgreSQL target is mandatory');
  assert.ok(['127.0.0.1', 'localhost', '[::1]'].includes(parsed.hostname), 'Acceptance requires the isolated loopback PostgreSQL forward');
  assert.match(decodeURIComponent(parsed.username), /^p01_test_[a-z0-9_]+$/, 'Refusing a PostgreSQL account outside the isolated test namespace');
  const database = decodeURIComponent(parsed.pathname.slice(1));
  assert.match(database, /^p01_test_[a-z0-9_]+$/, 'Refusing a database outside the p01_test_ namespace');
  assert.ok([...parsed.searchParams.keys()].every(key => key === 'sslmode'), 'Connection-file parameters cannot override the isolated target');
  return { connectionString, database };
}

export async function openPostgres(fileVariable) {
  const { connectionString, database } = dedicatedConnection(fileVariable);
  const pool = new pg.Pool({ connectionString, max: 3, connectionTimeoutMillis: 5_000,
    query_timeout: 10_000, statement_timeout: 10_000, lock_timeout: 5_000,
    application_name: 'p01_test_acceptance' });
  pool.on('error', () => {});
  try {
    const result = await pool.query('SELECT current_database() AS database, version() AS version');
    assert.equal(result.rows[0].database, database, 'Dedicated target identity changed');
    assert.match(result.rows[0].version, /^PostgreSQL 17\./, 'Acceptance requires PostgreSQL 17');
    return pool;
  } catch (error) { await pool.end(); throw error; }
}

export const businessColumns = Object.freeze({
  videos: ['id', 'name', 'width', 'height', 'duration_seconds', 'status', 'created_at', 'storage_key'],
  jobs: ['id', 'kind', 'video_id', 'batch_id', 'status', 'progress', 'error', 'created_at'],
  batches: ['id', 'video_id', 'job_id', 'created_at'],
  frames: ['id', 'video_id', 'batch_id', 'timestamp_ms', 'width', 'height', 'status', 'revision', 'storage_key'],
  annotations: ['id', 'frame_id', 'x', 'y', 'width', 'height', 'source'],
});

export async function businessRows(pool) {
  const out = {};
  for (const [table, columns] of Object.entries(businessColumns)) {
    const rows = (await pool.query(`SELECT ${columns.join(',')} FROM ${table} ORDER BY id`)).rows;
    out[table] = rows.map(row => columns.map(column => {
      const value = row[column];
      if (['revision', 'timestamp_ms'].includes(column)) {
        const number = Number(value);
        assert.ok(Number.isSafeInteger(number), 'Fixture integer is not losslessly representable');
        return number;
      }
      if (typeof value === 'number') assert.ok(Number.isFinite(value), 'Non-finite database value');
      return value;
    }));
  }
  return out;
}

export async function assertEmptyBusiness(pool) {
  const rows = await businessRows(pool);
  assert.ok(Object.values(rows).every(value => value.length === 0), 'Use a fresh isolated PostgreSQL business database');
}

export async function mediaReferenceRows(pool) {
  const out = {};
  for (const [table, columns] of Object.entries({
    videos: 'id,source_kind,source_id,storage_provider,storage_key,media_sha256,media_size_bytes',
    frames: 'id,video_id,batch_id,storage_provider,storage_key,media_sha256,media_size_bytes',
  })) out[table] = (await pool.query(`SELECT ${columns} FROM ${table} ORDER BY id`)).rows;
  return out;
}

export function sanitizedFailure(error) {
  return String(error?.message || error).replace(/postgres(?:ql)?:\/\/\S+/gi, '[redacted PostgreSQL URL]')
    .replace(/(?:password|token|api[_-]?key)\s*[=:]\s*[^\s,;]+/gi, '[redacted credential]');
}

export async function acceptance(main) {
  try { await main(); } catch (error) {
    console.error(`Acceptance failed: ${sanitizedFailure(error)}`);
    process.exitCode = 1;
  }
}
