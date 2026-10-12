import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';

function visit(directory) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (['node_modules', 'test-results', 'playwright-report'].includes(entry.name)) continue;
    const name = path.join(directory, entry.name);
    if (entry.isDirectory()) visit(name);
    else if (name.endsWith('.mjs')) {
      const result = spawnSync(process.execPath, ['--check', name], { stdio: 'inherit' });
      if (result.status !== 0) process.exit(result.status || 1);
    }
  }
}
visit('.');
console.log('Acceptance JavaScript syntax checked.');
