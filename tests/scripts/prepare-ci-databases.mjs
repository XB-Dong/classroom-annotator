import fs from 'node:fs';
import path from 'node:path';
import { randomUUID } from 'node:crypto';
import pg from 'pg';
import { acceptance, externalPath } from '../helpers/postgres.mjs';

await acceptance(async () => {
  const root = externalPath(process.env.P01_CI_CONFIG_DIR, 'P01_CI_CONFIG_DIR');
  fs.mkdirSync(root, { recursive: true, mode: 0o700 });
  const admin = new pg.Client({ connectionString: process.env.P01_CI_ADMIN_URL });
  await admin.connect();
  try {
    const password = randomUUID();
    // This administrative bootstrap is CI-only. Product tests never run as admin.
    await admin.query(`CREATE ROLE p01_test_owner LOGIN PASSWORD '${password}' NOSUPERUSER NOCREATEDB NOCREATEROLE`);
    for (const suffix of ['go', 'postgres', 'acceptance', 'import', 'rollback']) {
      const database = `p01_test_${suffix}`;
      await admin.query(`CREATE DATABASE ${database} OWNER p01_test_owner`);
      const url = new URL(process.env.P01_CI_ADMIN_URL);
      url.username = 'p01_test_owner'; url.password = password; url.pathname = `/${database}`;
      fs.writeFileSync(path.join(root, `${suffix}.url`), url.href, { mode: 0o600 });
    }
    console.log('Created isolated PostgreSQL databases with a non-superuser owner; connection files remain external.');
  } finally { await admin.end(); }
});
