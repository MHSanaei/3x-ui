// Converts the panel's TOML translation files (web/translation/) into
// nested JSON that vue-i18n can consume. The Go side stays the source
// of truth — translators continue to edit the TOML files; this script
// snapshots them into frontend/src/locales/<code>.json on each run.
//
// Run via `npm run i18n:sync` (also kicked off automatically by
// `npm run prebuild` so production builds always include the latest
// strings).
//
// Format support is intentionally narrow — the project's TOML files
// are limited to:
//   • blank lines and `# comment` lines
//   • bare-string values:   "key" = "value"
//   • dotted section heads:  [pages.inbounds.toasts]
// Multi-line strings, arrays, dates, and inline tables aren't used in
// the panel's translation set, so the parser rejects them rather than
// silently mis-parsing. If the format ever grows, swap this out for a
// proper TOML lib.

import { readdirSync, readFileSync, writeFileSync, mkdirSync, existsSync } from 'node:fs';
import { resolve, dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const tomlDir = resolve(here, '..', '..', 'web', 'translation');
const outDir = resolve(here, '..', 'src', 'locales');

if (!existsSync(outDir)) mkdirSync(outDir, { recursive: true });

// Decode the small set of escapes TOML allows inside basic strings.
// Unicode `\uXXXX` escapes aren't used in the panel's files but are
// handled too just in case a translator adds one.
function unescape(value) {
  return value.replace(/\\(["\\bfnrt]|u[0-9a-fA-F]{4})/g, (_m, what) => {
    if (what === '"') return '"';
    if (what === '\\') return '\\';
    if (what === 'b') return '\b';
    if (what === 'f') return '\f';
    if (what === 'n') return '\n';
    if (what === 'r') return '\r';
    if (what === 't') return '\t';
    return String.fromCharCode(parseInt(what.slice(1), 16));
  });
}

function setNested(target, path, value) {
  let cursor = target;
  for (let i = 0; i < path.length - 1; i++) {
    const seg = path[i];
    if (typeof cursor[seg] !== 'object' || cursor[seg] === null) {
      cursor[seg] = {};
    }
    cursor = cursor[seg];
  }
  cursor[path[path.length - 1]] = value;
}

const SECTION_RE = /^\[([A-Za-z0-9_.-]+)\]$/;
const KV_RE = /^"([^"\\]*(?:\\.[^"\\]*)*)"\s*=\s*"((?:[^"\\]|\\.)*)"$/;

function parseToml(src) {
  const tree = {};
  let section = [];
  let lineNo = 0;
  for (const rawLine of src.split(/\r?\n/)) {
    lineNo++;
    const line = rawLine.trim();
    if (!line || line.startsWith('#')) continue;

    const sectionMatch = SECTION_RE.exec(line);
    if (sectionMatch) {
      section = sectionMatch[1].split('.');
      continue;
    }

    const kvMatch = KV_RE.exec(line);
    if (!kvMatch) {
      throw new Error(`Unsupported TOML construct at line ${lineNo}: ${rawLine}`);
    }
    const [, key, value] = kvMatch;
    setNested(tree, [...section, unescape(key)], unescape(value));
  }
  return tree;
}

const files = readdirSync(tomlDir).filter((f) => f.startsWith('translate.') && f.endsWith('.toml'));
let count = 0;
for (const file of files) {
  const code = file.replace(/^translate\./, '').replace(/\.toml$/, '').replace('_', '-');
  const tree = parseToml(readFileSync(join(tomlDir, file), 'utf8'));
  const outPath = join(outDir, `${code}.json`);
  writeFileSync(outPath, JSON.stringify(tree, null, 2) + '\n');
  count++;
}

// eslint-disable-next-line no-console
console.log(`sync-locales: wrote ${count} locale file(s) to ${outDir}`);
