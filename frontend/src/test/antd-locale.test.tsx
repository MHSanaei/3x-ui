import { render } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

// Without a ConfigProvider locale, antd's own strings (empty states, close buttons,
// pagination) stayed English for every panel language (s-ui#1293).
describe('AntdRootConfig locale', () => {
  afterEach(() => {
    document.cookie = 'lang=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/';
  });

  it.each([
    ['fa-IR', 'داده‌ای موجود نیست'],
    ['en-US', 'No data'],
  ])('renders antd built-in text in the %s panel language', async (locale, emptyText) => {
    vi.resetModules();
    document.cookie = `lang=${locale}; path=/`;
    const { readyI18n } = await import('@/i18n/react');
    const { AntdRootConfig } = await import('@/i18n/AntdRootConfig');
    const { Empty } = await import('antd');

    await readyI18n('panel');
    render(
      <AntdRootConfig>
        <Empty />
      </AntdRootConfig>,
    );

    expect(document.querySelector('.ant-empty-description')?.textContent).toBe(emptyText);
  });
});
