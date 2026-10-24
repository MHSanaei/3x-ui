import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

// The s-ui#1293 icon guard walks oxlint's AST by hand, so an oxlint bump can kill
// it silently. These tests fail loudly when it does.
const FIXTURES = 'tools/oxlint/__fixtures__';
const RULE = 'antd-icon(explicit-a11y)';

function runGuard(target: string): string {
  try {
    execFileSync(
      process.execPath,
      [
        'node_modules/oxlint/bin/oxlint',
        '-c',
        `${FIXTURES}/icon.oxlintrc.json`,
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

describe('icon-a11y-guard oxlint plugin', () => {
  it('flags Ant Design icons that would announce their English name', () => {
    const hits = runGuard(`${FIXTURES}/icon-unmarked.tsx`)
      .split('\n')
      .filter((line) => line.includes(RULE))
      .map((line) => Number(line.split(':')[1]));

    // The button icon and the standalone icon; a same-suffix non-antd component is ignored.
    expect(hits).toEqual([6, 7]);
  });

  it('accepts icons marked aria-hidden, named, labelled-by or given forwarded props', () => {
    expect(runGuard(`${FIXTURES}/icon-marked.tsx`)).not.toContain(RULE);
  });
});

describe('icon-a11y-guard wiring', () => {
  const config = JSON.parse(readFileSync('.oxlintrc.json', 'utf8')) as {
    jsPlugins: string[];
    overrides: { files: string[]; rules: Record<string, unknown> }[];
  };

  it('enforces the rule on app sources but not on stories or tests', () => {
    expect(config.jsPlugins).toContain('./tools/oxlint/icon-a11y-guard.mjs');
    const enabled = config.overrides.find((o) => o.rules['antd-icon/explicit-a11y'] === 'error');
    expect(enabled?.files).toEqual(['src/**/*.tsx']);
    const exempt = config.overrides.find((o) => o.rules['antd-icon/explicit-a11y'] === 'off');
    expect(exempt?.files).toEqual(['src/**/*.stories.tsx', 'src/test/**']);
  });
});
