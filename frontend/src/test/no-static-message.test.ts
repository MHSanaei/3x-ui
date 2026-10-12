import { readFileSync, readdirSync, statSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { join, relative, resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

const srcRoot = resolve(fileURLToPath(import.meta.url), '../..');
const staticCall = /(?<![\w.$])message\.(success|error|warning|info|loading|open)\(/;

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return name === 'test' ? [] : sourceFiles(path);
    return /\.tsx?$/.test(name) ? [path] : [];
  });
}

// antd's static message renders outside React: it ignores the theme and its
// timers outlive the component, which broke CI after the Happ tests tore down.
describe('antd message', () => {
  it('is only used through message.useMessage()', () => {
    const offenders = sourceFiles(srcRoot).flatMap((file) =>
      readFileSync(file, 'utf8')
        .split('\n')
        .flatMap((line, i) =>
          staticCall.test(line) ? [`${relative(srcRoot, file)}:${i + 1}: ${line.trim()}`] : [],
        ),
    );
    expect(offenders).toEqual([]);
  });
});
