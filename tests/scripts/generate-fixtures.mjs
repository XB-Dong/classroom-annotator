import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const root = path.resolve(fileURLToPath(new URL('../..', import.meta.url)));
if (!process.env.P0_FIXTURE_DIR) throw new Error('P0_FIXTURE_DIR must name a directory outside the repository.');
const output = path.resolve(process.env.P0_FIXTURE_DIR);
const relative = path.relative(root, output);
if (!relative.startsWith('..') && !path.isAbsolute(relative)) throw new Error('Fixture directory must be outside the repository.');
fs.mkdirSync(output, { recursive: true });
const result = spawnSync(process.env.FFMPEG_PATH || 'ffmpeg', [
  '-nostdin', '-hide_banner', '-loglevel', 'error', '-y', '-f', 'lavfi', '-i',
  'color=c=0x223344:s=320x240:r=25:d=4', '-vf',
  'drawbox=x=60:y=40:w=30:h=30:color=yellow:t=fill,drawbox=x=180:y=70:w=24:h=24:color=cyan:t=fill',
  '-an', '-c:v', 'mpeg4', '-q:v', '3', '-threads', '1', path.join(output, 'classroom.mp4'),
], { encoding: 'utf8' });
if (result.error) throw result.error;
if (result.status !== 0) throw new Error(`FFmpeg exit ${result.status}: ${result.stderr}`);
fs.writeFileSync(path.join(output, 'corrupt.mp4'), 'synthetic invalid media');
console.log('Generated synthetic 320x240 / 4s / 25fps fixtures outside the repository.');
