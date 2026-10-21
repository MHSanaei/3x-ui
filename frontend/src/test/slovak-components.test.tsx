import { afterEach, describe, expect, it } from 'vitest';
import { act, cleanup, render, screen } from '@testing-library/react';
import { ConfigProvider, DatePicker, Empty, Table } from 'antd';
import dayjs from 'dayjs';
import i18next from 'i18next';

import { LocaleProvider } from '@/i18n/LocaleProvider';
import skSK from '../../../internal/web/translation/sk-SK.json';

i18next.addResourceBundle('sk-SK', 'translation', skSK, true, true);

afterEach(async () => {
  cleanup();
  await act(async () => {
    await i18next.changeLanguage('en-US');
  });
});

describe('Slovak component locale', () => {
  it('localizes the calendar and empty tables through nested page providers', async () => {
    await i18next.changeLanguage('sk-SK');
    render(
      <LocaleProvider>
        <ConfigProvider theme={{ hashed: false, token: { motion: false } }}>
          <DatePicker open value={dayjs('2026-10-21')} />
          <Table columns={[]} dataSource={[]} />
        </ConfigProvider>
      </LocaleProvider>,
    );
    expect(await screen.findByText('Dnes')).toBeTruthy();
    expect(screen.getByText('Žiadne dáta', { selector: '.ant-empty-description' })).toBeTruthy();
  });

  it('preserves the default component locale for existing languages', async () => {
    await i18next.changeLanguage('en-US');
    render(
      <LocaleProvider>
        <Empty />
      </LocaleProvider>,
    );
    expect(screen.getByText('No data', { selector: '.ant-empty-description' })).toBeTruthy();
  });
});
