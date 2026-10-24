import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

// The s-ui#1293 unnamed-control guard walks oxlint's AST by hand, so an oxlint
// bump can kill it silently. These tests fail loudly when it does.
const FIXTURES = 'tools/oxlint/__fixtures__';
const RULE = 'control-label(has-name)';

function runGuard(target: string): string {
  try {
    execFileSync(
      process.execPath,
      [
        'node_modules/oxlint/bin/oxlint',
        '-c',
        `${FIXTURES}/label.oxlintrc.json`,
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

describe('control-label-guard oxlint plugin', () => {
  it('flags every control a screen reader would announce without a name', () => {
    const hits = runGuard(`${FIXTURES}/unlabeled-control.tsx`)
      .split('\n')
      .filter((line) => line.includes(RULE))
      .map((line) => Number(line.split(':')[1]));

    // Bare cell switch, label-only Form.Item, id without htmlFor, htmlFor without
    // id, nested under a named Form.Item, nested under SettingListItem.
    expect(hits).toEqual([5, 9, 15, 21, 28, 36]);
  });

  it('accepts aria names, wrapping labels, htmlFor/id pairs and the labelling wrappers', () => {
    expect(runGuard(`${FIXTURES}/labelled-control.tsx`)).not.toContain(RULE);
  });
});

describe('control-label-guard wiring', () => {
  const config = JSON.parse(readFileSync('.oxlintrc.json', 'utf8')) as {
    jsPlugins: string[];
    overrides: { files: string[]; rules: Record<string, unknown> }[];
  };

  it('enforces the rule on app sources but not on stories or tests', () => {
    expect(config.jsPlugins).toContain('./tools/oxlint/control-label-guard.mjs');
    const enabled = config.overrides.find((o) => o.rules['control-label/has-name'] === 'error');
    expect(enabled?.files).toEqual(['src/**/*.tsx']);
    const exempt = config.overrides.find((o) => o.rules['control-label/has-name'] === 'off');
    expect(exempt?.files).toEqual(['src/**/*.stories.tsx', 'src/test/**']);
  });
});
