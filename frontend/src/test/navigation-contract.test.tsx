import { readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

import { PALETTE_KEYWORDS } from '@/components/command-palette/CommandPalette';
import { PANEL_ROUTES, PANEL_ROUTE_PATHS } from '@/routes/panelRoutes';
import { routes } from '@/routes';

// Four hand-kept nav lists let a page go missing from the sidebar or fall back to the "3X-UI"
// title; the manifest replaced them, and these tests hold it to the router and the bundle.

const translationDir = resolve(process.cwd(), '..', 'internal', 'web', 'translation');

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

describe('panel navigation', () => {
  it('lists every route once', () => {
    expect(new Set(PANEL_ROUTE_PATHS).size).toBe(PANEL_ROUTE_PATHS.length);
  });

  it('routes every manifest path', () => {
    const childPaths = (routes[0].children ?? []).map((child) =>
      child.index ? '/' : `/${child.path ?? ''}`,
    );
    expect(childPaths.sort()).toEqual([...PANEL_ROUTE_PATHS].sort());
  });

  it('translates every title key', () => {
    const enKeys = new Set(
      flattenKeys(
        JSON.parse(readFileSync(join(translationDir, 'en-US.json'), 'utf8')) as Record<
          string,
          unknown
        >,
      ),
    );
    for (const route of PANEL_ROUTES) {
      expect(enKeys.has(route.titleKey)).toBe(true);
    }
  });

  it('gives every route command-palette keywords', () => {
    expect(Object.keys(PALETTE_KEYWORDS).sort()).toEqual([...PANEL_ROUTE_PATHS].sort());
  });
});
