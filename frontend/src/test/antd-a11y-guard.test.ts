import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

// The Spin/Select screen-reader guards walk oxlint's AST by hand, so an oxlint
// bump can kill them silently. These tests fail loudly when it does.
const FIXTURES = 'tools/oxlint/__fixtures__';
const SPIN_RULE = 'antd-a11y(spin-content-not-live)';
const SELECT_RULE = 'antd-a11y(select-tag-remove)';

function runGuard(target: string): string {
  try {
    execFileSync(
      process.execPath,
      [
        'node_modules/oxlint/bin/oxlint',
        '-c',
        `${FIXTURES}/antd-a11y.oxlintrc.json`,
        '-f',
        'unix',
        target,
      ],
      { encoding: 'utf8', stdio: 'pipe' },
    );
    return '';
  } catch (error) {
    const { status, stdout } = error as { status?: number | null; stdout?: string };
    if (typeof status !== 'number') throw error;
    return String(stdout ?? '');
  }
}

function hitLines(output: string, rule: string): number[] {
  return output
    .split('\n')
    .filter((line) => line.includes(rule))
    .map((line) => Number(line.split(':')[1]));
}

describe('antd-a11y-guard oxlint plugin', () => {
  it('flags a Spin whose content would be announced on every update', () => {
    expect(hitLines(runGuard(`${FIXTURES}/antd-a11y-flagged.tsx`), SPIN_RULE)).toEqual([5]);
  });

  it('flags tag Selects without a focusable remove button or with hidden tags', () => {
    expect(hitLines(runGuard(`${FIXTURES}/antd-a11y-flagged.tsx`), SELECT_RULE)).toEqual([6, 7]);
  });

  it('accepts quiet Spins, bare indicators, single Selects, tagRender and spreads', () => {
    const output = runGuard(`${FIXTURES}/antd-a11y-ok.tsx`);
    expect(output).not.toContain(SPIN_RULE);
    expect(output).not.toContain(SELECT_RULE);
  });
});

describe('antd-a11y-guard wiring', () => {
  const config = JSON.parse(readFileSync('.oxlintrc.json', 'utf8')) as {
    jsPlugins: string[];
    overrides: { files: string[]; rules: Record<string, unknown> }[];
  };

  it('enforces both rules on app sources but not on stories or tests', () => {
    expect(config.jsPlugins).toContain('./tools/oxlint/antd-a11y-guard.mjs');
    for (const rule of ['antd-a11y/spin-content-not-live', 'antd-a11y/select-tag-remove']) {
      expect(config.overrides.find((o) => o.rules[rule] === 'error')?.files).toEqual([
        'src/**/*.tsx',
      ]);
      expect(config.overrides.find((o) => o.rules[rule] === 'off')?.files).toEqual([
        'src/**/*.stories.tsx',
        'src/test/**',
      ]);
    }
  });
});
