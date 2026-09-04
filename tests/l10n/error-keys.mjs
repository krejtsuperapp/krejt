// Çdo çelës mesazhi gabimi që kthen serveri (message_key te httpx.APIError) duhet të ketë
// përkthim në të tria gjuhët e aplikacioneve. Pa të, përdoruesi sheh "Diçka shkoi keq" edhe
// kur serveri e di saktësisht çfarë ndodhi (p.sh. kod OTP i gabuar). Ky kontroll ekzekutohet
// në CI dhe dështon me listën e çelësave që mungojnë.
//
//   node tests/l10n/error-keys.mjs
import fs from 'node:fs';
import path from 'node:path';

const root = path.resolve(import.meta.dirname, '..', '..');

function walk(dir, out = []) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walk(p, out);
    else if (e.name.endsWith('.go') && !e.name.endsWith('_test.go')) out.push(p);
  }
  return out;
}

const serverKeys = new Set();
for (const file of walk(path.join(root, 'backend', 'internal'))) {
  const src = fs.readFileSync(file, 'utf8');
  for (const m of src.matchAll(/MessageKey:\s*"(errors\.[a-z_.]+)"/g)) serverKeys.add(m[1]);
}

const strings = fs.readFileSync(
  path.join(root, 'packages', 'krejt_l10n', 'lib', 'src', 'strings.dart'),
  'utf8',
);
// Tabelat e gjuhëve: `const Map<String, String> _sq = {` (dhe _en, _de) — çelësat numërohen
// brenda secilës.
const perLang = new Map();
let lang = null;
for (const line of strings.split('\n')) {
  const table = line.match(/^const Map<String, String> _([a-z]{2}) = \{/);
  if (table) {
    lang = table[1];
    perLang.set(lang, new Set());
    continue;
  }
  if (!lang) continue;
  const key = line.match(/^\s*'(errors\.[a-z_.]+)':/);
  if (key) perLang.get(lang).add(key[1]);
}

const langs = [...perLang.keys()];
if (langs.length < 3) {
  console.error(`u gjetën vetëm ${langs.length} tabela gjuhësh te strings.dart (${langs.join(', ')})`);
  process.exit(1);
}

const missing = [];
for (const key of [...serverKeys].sort()) {
  for (const l of langs) {
    if (!perLang.get(l).has(key)) missing.push(`${key} (${l})`);
  }
}

if (missing.length > 0) {
  console.error(`mungojnë ${missing.length} përkthime të çelësave të serverit:\n  ${missing.join('\n  ')}`);
  process.exit(1);
}
console.log(`${serverKeys.size} çelësa gabimi të serverit, të gjithë të përkthyer në ${langs.join('/')}`);
