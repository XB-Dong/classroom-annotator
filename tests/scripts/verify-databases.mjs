import assert from 'node:assert/strict';
import { acceptance, dedicatedConnection, openPostgres } from '../helpers/postgres.mjs';

await acceptance(async () => {
  const variables = ['P01_GO_DATABASE_URL_FILE', 'P01_DATABASE_URL_FILE', 'P01_ACCEPTANCE_DATABASE_URL_FILE',
    'P01_IMPORT_DATABASE_URL_FILE', 'P01_ROLLBACK_DATABASE_URL_FILE'];
  const names = variables.map(variable => dedicatedConnection(variable).database);
  assert.equal(new Set(names).size, names.length, 'Acceptance tasks require independent databases');
  assert.equal(names[0], 'p01_test_go', 'Go integration requires p01_test_go');
  for (const variable of variables) {
    const pool = await openPostgres(variable);
    try {
      const role = (await pool.query('SELECT rolsuper,rolcreatedb,rolcreaterole FROM pg_roles WHERE rolname=current_user')).rows[0];
      assert.deepEqual(role, { rolsuper: false, rolcreatedb: false, rolcreaterole: false }, 'Test owner must have no administrative privileges');
    } finally { await pool.end(); }
  }
  console.log('Mandatory independent PostgreSQL 17 targets and non-superuser identities verified.');
});
