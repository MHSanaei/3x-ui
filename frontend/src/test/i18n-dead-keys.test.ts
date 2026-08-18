import { readFileSync, readdirSync, statSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { join, resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

/*
 * Guards the 14-locale translation set two ways: every key in en-US must be
 * referenced somewhere in the frontend or Go sources (dead keys accumulate
 * silently — this test deleted over two hundred of them when it was
 * introduced), and every locale must carry exactly the en-US key set
 * (missing keys fall back to en-US at runtime, so nothing else fails the
 * build when a translation is forgotten).
 *
 * References are matched as whole dotted tokens, not substrings, so a dead
 * key cannot hide behind a longer live sibling. Dynamically built keys are
 * covered by harvesting the string-literal prefixes that appear next to
 * concatenation or template-literal interpolation; the prefix needs at
 * least one dot but need not end on one, so a literal that stops mid-leaf
 * right before the interpolation still keeps its subtree alive. This file
 * excludes itself from the scan so the prose above cannot whitelist
 * anything.
 */

const repoRoot = resolve(process.cwd(), '..');
const translationDir = join(repoRoot, 'internal', 'web', 'translation');
const selfPath = fileURLToPath(import.meta.url);

function flattenKeys(obj: Record<string, unknown>, prefix = ''): string[] {
  const keys: string[] = [];
  for (const [k, v] of Object.entries(obj)) {
    const key = `${prefix}${k}`;
    if (v !== null && typeof v === 'object') {
      keys.push(...flattenKeys(v as Record<string, unknown>, `${key}.`));
    } else {
      keys.push(key);
    }
  }
  return keys;
}

function collectSources(dir: string, exts: string[], out: string[]): void {
  for (const entry of readdirSync(dir)) {
    if (['node_modules', 'dist', 'generated', '.git', '.local', 'storybook-static'].includes(entry)) continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      collectSources(full, exts, out);
    } else if (exts.some((ext) => entry.endsWith(ext)) && resolve(full) !== selfPath) {
      out.push(readFileSync(full, 'utf8'));
    }
  }
}

describe('i18n keys', () => {
  const enUS = JSON.parse(readFileSync(join(translationDir, 'en-US.json'), 'utf8'));
  const enKeys = flattenKeys(enUS);

  const sources: string[] = [];
  collectSources(join(repoRoot, 'frontend', 'src'), ['.ts', '.tsx'], sources);
  collectSources(join(repoRoot, 'internal'), ['.go'], sources);
  const blob = sources.join('\n');

  const tokens = new Set(blob.match(/[A-Za-z][A-Za-z0-9_.]*/g) ?? []);

  const prefixes: string[] = [];
  for (const match of blob.matchAll(/['"`]([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z0-9_]+)+\.?)['"`]\s*\+/g)) {
    prefixes.push(match[1]);
  }
  for (const match of blob.matchAll(/[`']([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z0-9_]+)+\.?)\$\{/g)) {
    prefixes.push(match[1]);
  }

  it('every en-US key is referenced by the frontend or Go sources', () => {
    const dead = enKeys.filter(
      (key) => !tokens.has(key) && !prefixes.some((p) => key.startsWith(p)),
    );
    expect(dead, `dead i18n keys (delete from all 14 locales):\n  ${dead.join('\n  ')}`).toEqual([]);
  });

  it('every locale carries exactly the en-US key set', () => {
    const enSet = new Set(enKeys);
    for (const file of readdirSync(translationDir)) {
      if (!file.endsWith('.json') || file === 'en-US.json') continue;
      const keys = new Set(flattenKeys(JSON.parse(readFileSync(join(translationDir, file), 'utf8'))));
      const missing = enKeys.filter((k) => !keys.has(k));
      const orphans = [...keys].filter((k) => !enSet.has(k));
      expect(missing, `${file} is missing keys:\n  ${missing.join('\n  ')}`).toEqual([]);
      expect(orphans, `${file} has keys absent from en-US:\n  ${orphans.join('\n  ')}`).toEqual([]);
    }
  });
});
