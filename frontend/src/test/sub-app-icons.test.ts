import { describe, expect, it } from 'vitest';

import { APP_ICONS } from '@/pages/sub/appIcons';
import { buildSubApps } from '@/pages/sub/subPageModel';

describe('APP_ICONS', () => {
  it('has an icon for every app the subscription page offers on every platform', () => {
    const apps = buildSubApps({
      subUrl: 'https://sub.example.com/sub/abc',
      sId: 'abc',
      subTitle: '',
    });
    const names = [...new Set(Object.values(apps).flatMap((list) => list.map((app) => app.name)))];

    expect(names.filter((name) => !APP_ICONS[name]?.src)).toEqual([]);
  });

  it('carries no icon for an app the page no longer offers', () => {
    const apps = buildSubApps({
      subUrl: 'https://sub.example.com/sub/abc',
      sId: 'abc',
      subTitle: '',
    });
    const offered = new Set(Object.values(apps).flatMap((list) => list.map((app) => app.name)));

    expect(Object.keys(APP_ICONS).filter((name) => !offered.has(name))).toEqual([]);
  });
});
