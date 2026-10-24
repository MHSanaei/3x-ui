import { readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

import { PALETTE_KEYWORDS } from '@/components/command-palette/CommandPalette';
import { PANEL_ROUTES, PANEL_ROUTE_PATHS, PANEL_TITLE_KEYS } from '@/routes/panelRoutes';
import { routes } from '@/routes';

/*
 * The panel's navigation used to be four hand-kept lists: the router, the
 * sidebar, the page titles and the command palette. A page added to one of them
 * was silently missing from the others - no nav entry, or a page title that
 * fell back to "3X-UI". They all read one manifest now, and this test holds the
 * manifest to the router and to the translation bundle.
 */

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

  it('titles every manifest path', () => {
    expect(Object.keys(PANEL_TITLE_KEYS).sort()).toEqual([...PANEL_ROUTE_PATHS].sort());
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
