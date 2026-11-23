// A release is not done without its screenshots (RELEASE-POLICY §5d): 3–4 PNGs in docs/screenshots,
// each under 400 KB, each named <screen>-<light|dark>.png and referenced from README.md.
import { readdir, readFile, stat } from 'node:fs/promises';
import { join } from 'node:path';

const dir = 'docs/screenshots';
const LIMIT = 400 * 1024;
const problems = [];

let names = [];
try {
  names = (await readdir(dir)).filter(name => name.endsWith('.png')).sort();
} catch {
  problems.push(`${dir} does not exist`);
}

if (names.length === 0) problems.push(`${dir} holds no screenshots`);
if (names.length > 0 && names.length < 3) problems.push(`${dir} holds ${names.length}; a release needs at least 3`);
if (names.length > 4) problems.push(`${dir} holds ${names.length}; the policy asks for 3–4`);

const readme = await readFile('README.md', 'utf8');
for (const name of names) {
  if (!/^[a-z0-9-]+-(light|dark)\.png$/.test(name)) problems.push(`${name} is not named <screen>-<light|dark>.png`);
  const { size } = await stat(join(dir, name));
  if (size > LIMIT) problems.push(`${name} is ${size} bytes, over the 400 KB limit`);
  if (!readme.includes(`${dir}/${name}`)) problems.push(`${name} is not referenced from README.md`);
}

if (problems.length) {
  for (const problem of problems) console.error(`FAIL: ${problem}`);
  process.exit(1);
}
console.log(`Screenshots: ${names.length} files, each under 400 KB and referenced from README.md`);