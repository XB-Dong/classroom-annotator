import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { DatabaseSync } from 'node:sqlite';
import { acceptance, externalPath, dedicatedConnection, openPostgres, businessColumns, businessRows, assertEmptyBusiness } from '../helpers/postgres.mjs';
import { command } from '../helpers/process.mjs';

const hash = value => createHash('sha256').update(value).digest('hex');

function legacyFixture(filename) {
  const db = new DatabaseSync(filename);
  try {
    db.exec('PRAGMA foreign_keys=ON');
    db.exec(fs.readFileSync(new URL('../fixtures/legacy-schema.sql', import.meta.url), 'utf8'));
    const values = {
      videos: [['legacy-video-1', 'Synthetic legacy', 320, 240, 20, 'ready', '2026-01-01T00:00:00Z', 'videos/legacy-1.mp4']],
      jobs: [
        ['legacy-job-1', 'extraction', 'legacy-video-1', 'legacy-batch-1', 'completed', 1, '', '2026-01-01T00:00:01Z'],
        ['legacy-job-2', 'extraction', 'legacy-video-1', 'legacy-batch-2', 'completed', 1, '', '2026-01-01T00:00:02Z'],
        ['legacy-job-interrupted', 'extraction', 'legacy-video-1', 'legacy-batch-unpublished', 'running', 0.125, '', '2026-01-01T00:00:03Z'],
        ['legacy-job-failed', 'import', null, null, 'failed', 0, 'SYNTHETIC_FAILURE', '2026-01-01T00:00:04Z'],
      ],
      batches: [
        ['legacy-batch-1', 'legacy-video-1', 'legacy-job-1', '2026-01-01T00:00:01Z'],
        ['legacy-batch-2', 'legacy-video-1', 'legacy-job-2', '2026-01-01T00:00:02Z'],
      ],
      frames: [
        ['legacy-frame-1', 'legacy-video-1', 'legacy-batch-1', 0, 320, 240, 'completed', 7, 'frames/legacy-batch-1/0.jpg'],
        ['legacy-frame-2', 'legacy-video-1', 'legacy-batch-1', 2000, 320, 240, 'completed', 3, 'frames/legacy-batch-1/1.jpg'],
        ['legacy-frame-3', 'legacy-video-1', 'legacy-batch-1', 4000, 320, 240, 'in_progress', 11, 'frames/legacy-batch-1/2.jpg'],
        ['legacy-frame-4', 'legacy-video-1', 'legacy-batch-2', 0, 320, 240, 'not_started', 0, 'frames/legacy-batch-2/0.jpg'],
      ],
      annotations: [
        ['legacy-ann-1', 'legacy-frame-1', 0.1, 12.125, 20.333333333333332, 30.25, 'manual'],
        ['legacy-ann-2', 'legacy-frame-1', 100, 90, 13.5, 17.75, 'manual'],
        ['legacy-ann-3', 'legacy-frame-3', 23.45678901234567, 34.56789012345678, 50.125, 60.25, 'manual'],
      ],
    };
    for (const [table, columns] of Object.entries(businessColumns)) {
      const insert = db.prepare(`INSERT INTO ${table} (${columns.join(',')}) VALUES (${columns.map(() => '?').join(',')})`);
      for (const row of values[table]) insert.run(...row);
    }
    assert.equal(db.prepare('PRAGMA integrity_check').get().integrity_check, 'ok');
    assert.deepEqual(db.prepare('PRAGMA foreign_key_check').all(), []);
  } finally { db.close(); }
}

function sourceRows(filename) {
  const db = new DatabaseSync(filename, { readOnly: true });
  try {
    return Object.fromEntries(Object.entries(businessColumns).map(([table, columns]) => [table,
      db.prepare(`SELECT ${columns.join(',')} FROM ${table} ORDER BY id`).all().map(row => columns.map(column => row[column])),
    ]));
  } finally { db.close(); }
}

await acceptance(async () => {
  const root = externalPath(process.env.P01_MIGRATION_OUTPUT, 'P01_MIGRATION_OUTPUT');
  assert.ok(!fs.existsSync(root), 'Use a fresh external migration evidence directory');
  fs.mkdirSync(root, { recursive: true });
  const source = path.join(root, 'legacy-backup.sqlite');
  legacyFixture(source);
  const sourceHash = hash(fs.readFileSync(source));
  const expected = sourceRows(source);
  const targets = ['P01_IMPORT_DATABASE_URL_FILE', 'P01_ROLLBACK_DATABASE_URL_FILE'];
  assert.notEqual(dedicatedConnection(targets[0]).database, dedicatedConnection(targets[1]).database, 'Import and rollback require distinct databases');
  const evidence = { sourceSHA: process.env.P0_EXPECTED_SHA, observedAt: new Date().toISOString(), sourceBackupSHA256: sourceHash, tables: {}, commands: [] };
  for (const [index, variable] of targets.entries()) {
    const pool = await openPostgres(variable);
    const environment = { APP_DATABASE_URL: dedicatedConnection(variable).connectionString };
    try {
      evidence.commands.push(await command(process.env.P01_MIGRATE_BINARY, ['up'], environment));
      await assertEmptyBusiness(pool);
      const version = (await pool.query('SELECT version_id,is_applied FROM goose_db_version ORDER BY id')).rows;
      assert.ok(version.some(row => Number(row.version_id) >= 2 && row.is_applied), 'Expected provider schema migration not applied');
      evidence.commands.push(await command(process.env.P01_MIGRATE_BINARY, ['up'], environment));
      assert.deepEqual((await pool.query('SELECT version_id,is_applied FROM goose_db_version ORDER BY id')).rows, version, 'Repeated migration changed installed versions');
      evidence.commands.push(await command(process.env.P01_MIGRATE_BINARY, ['status'], environment));
      const report = path.join(root, index ? 'rollback-report.json' : 'import-report.json');
      if (index === 1) {
        await pool.query(`CREATE FUNCTION p01_test_reject_frame() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.id='legacy-frame-3' THEN RAISE EXCEPTION 'SYNTHETIC_IMPORT_FAILURE'; END IF; RETURN NEW; END $$`);
        await pool.query('CREATE TRIGGER p01_test_reject_frame BEFORE INSERT ON frames FOR EACH ROW EXECUTE FUNCTION p01_test_reject_frame()');
        try {
          evidence.commands.push(await command(process.env.P01_IMPORT_BINARY, ['--source', source, '--report', report], environment, 'failure'));
          await assertEmptyBusiness(pool);
        } finally {
          await pool.query('DROP TRIGGER p01_test_reject_frame ON frames');
          await pool.query('DROP FUNCTION p01_test_reject_frame()');
        }
      } else {
        evidence.commands.push(await command(process.env.P01_IMPORT_BINARY, ['--source', source, '--report', report], environment));
        assert.deepEqual(await businessRows(pool), expected, 'Imported fields differ from the read-only SQLite backup');
        assert.ok(fs.existsSync(report), 'Importer did not write its requested external report');
        const before = await businessRows(pool);
        evidence.commands.push(await command(process.env.P01_IMPORT_BINARY, ['--source', source, '--report', path.join(root, 'refused-report.json')], environment, 'failure'));
        assert.deepEqual(await businessRows(pool), before, 'Refused nonempty target was changed');
        for (const [table, rows] of Object.entries(before)) evidence.tables[table] = { count: rows.length, sha256: hash(JSON.stringify(rows)) };
        const provider = (await pool.query('SELECT source_kind,storage_provider FROM videos')).rows;
        assert.deepEqual(provider, [{ source_kind: 'legacy', storage_provider: 'local' }]);
      }
      assert.equal(hash(fs.readFileSync(source)), sourceHash, 'SQLite backup changed during import/refusal/rollback');
    } finally { await pool.end(); }
  }
  fs.writeFileSync(path.join(root, 'migration-evidence.json'), JSON.stringify(evidence, null, 2));
  console.log('P01-DB-001/006 passed: actual PostgreSQL migrations, faithful five-table import, nonempty refusal, full rollback, unchanged source backup.');
});
