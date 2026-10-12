import fs from 'node:fs';
import Ajv from 'ajv/dist/2020.js';
import formats from 'ajv-formats';
import assert from 'node:assert/strict';

const contract = JSON.parse(fs.readFileSync(new URL('../../specs/contracts/openapi.yaml', import.meta.url)));
const ajv = new Ajv({ strict: false, allErrors: true });
formats(ajv);
ajv.addSchema({ $id: 'p0', ...contract });

export function validateSchema(name, value) {
  const validate = ajv.getSchema(`p0#/components/schemas/${name}`);
  assert.ok(validate(value), JSON.stringify(validate.errors));
  if (name === 'Job' && value.kind === 'extraction') {
    assert.equal(typeof value.videoId, 'string');
    assert.equal(typeof value.batchId, 'string');
  }
  return value;
}
