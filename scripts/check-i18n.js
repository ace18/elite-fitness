// Fails if the locale catalogues drift apart.
//
// A missing key doesn't crash — translate() falls back to English and then to
// the key itself — so without this check a forgotten Italian string would just
// quietly render in English. Run with `npm run check:i18n`.

import { en } from '../src/lib/i18n/en.js';
import { it } from '../src/lib/i18n/it.js';

function flatten(obj, prefix = '') {
  const out = [];
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k;
    if (v && typeof v === 'object') out.push(...flatten(v, key));
    else out.push(key);
  }
  return out;
}

const enKeys = new Set(flatten(en));
const itKeys = new Set(flatten(it));

const missingInIt = [...enKeys].filter((k) => !itKeys.has(k));
const missingInEn = [...itKeys].filter((k) => !enKeys.has(k));

// Placeholders must match too: {n} in one locale and {count} in the other
// renders a literal "{count}" to the user.
const placeholderMismatch = [];
const vars = (s) => (s.match(/\{(\w+)\}/g) ?? []).sort().join(',');
const get = (obj, key) => key.split('.').reduce((o, p) => o?.[p], obj);
for (const key of enKeys) {
  if (!itKeys.has(key)) continue;
  const a = vars(get(en, key)), b = vars(get(it, key));
  if (a !== b) placeholderMismatch.push(`${key}  en:[${a}]  it:[${b}]`);
}

let failed = false;
const report = (label, list) => {
  if (!list.length) return;
  failed = true;
  console.error(`\n${label} (${list.length}):`);
  for (const k of list) console.error(`  · ${k}`);
};

report('Missing in it.js', missingInIt);
report('Missing in en.js', missingInEn);
report('Placeholder mismatch', placeholderMismatch);

if (failed) process.exit(1);
console.log(`i18n OK — ${enKeys.size} keys, both locales in sync`);
