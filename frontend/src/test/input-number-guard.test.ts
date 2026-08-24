import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

// The #6121/#6127 guard is hand-written JS walking oxlint's AST, so it can go
// silently dead on an oxlint bump. These tests fail loudly when it does.
const FIXTURES = 'tools/oxlint/__fixtures__';
const RULE = 'input-number(no-synthetic-clear)';

function runGuard(target: string): string {
  try {
    execFileSync('./node_modules/.bin/oxlint', ['-c', `${FIXTURES}/guard.oxlintrc.json`, target], {
      encoding: 'utf8',
      stdio: 'pipe',
    });
    return '';
  } catch (error) {
    return String((error as { stdout?: string }).stdout ?? '');
  }
}

describe('input-number-guard oxlint plugin', () => {
  it('rejects every shape that turns a cleared InputNumber into a stored number', () => {
    const output = runGuard(`${FIXTURES}/synthetic-clear.tsx`);
    const hits = output.split('\n').filter((line) => line.includes(RULE));

    // Number(v) || N, the typeof ternary, and v ?? N.
    expect(hits).toHaveLength(3);
    expect(output).toContain('#6127');
  });

  it('accepts a handler wrapped with onNumber()', () => {
    expect(runGuard(`${FIXTURES}/ok.tsx`)).not.toContain(RULE);
  });
});

describe('input-number-guard wiring', () => {
  const config = JSON.parse(readFileSync('.oxlintrc.json', 'utf8')) as {
    jsPlugins: string[];
    rules: Record<string, unknown>;
    overrides: { files: string[]; rules: Record<string, unknown> }[];
  };

  it('loads the plugin and scopes the rule to the pages that regressed', () => {
    expect(config.jsPlugins).toContain('./tools/oxlint/input-number-guard.mjs');
    expect(config.rules['input-number/no-synthetic-clear']).toBe('off');

    const enabled = config.overrides.find(
      (o) => o.rules['input-number/no-synthetic-clear'] === 'error',
    );
    expect(enabled?.files).toEqual(['src/pages/settings/**/*.tsx', 'src/pages/xray/**/*.tsx']);

    const exempt = config.overrides.find(
      (o) => o.rules['input-number/no-synthetic-clear'] === 'off',
    );
    expect(exempt?.files).toEqual(['src/pages/xray/**/*Modal.tsx']);
  });
});
