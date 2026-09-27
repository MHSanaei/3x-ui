import { expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';
import { FormProvider, useForm } from 'react-hook-form';
import { createInstance } from 'i18next';
import { I18nextProvider } from 'react-i18next';

import ClientRenewalFields from '@/pages/clients/ClientRenewalFields';
import { HttpUtil, Msg } from '@/utils';
import zhCN from '../../../internal/web/translation/zh-CN.json';
import { renderWithProviders } from './test-utils';

function PreviewForm() {
  const form = useForm({ defaultValues: { reset: 1, resetDay: 0, resetWeekday: 0, resetMax: 0 } });
  return (
    <FormProvider {...form}>
      <ClientRenewalFields active expiryTime={1790485200000} setExpiry={() => {}} />
    </FormProvider>
  );
}

it.each([
  {
    name: 'explains Local UTC without converting into the browser timezone',
    zone: 'Local',
    label: 'Server local time',
    dates: ['2026-09-27T05:00:00Z', '2026-09-27T04:59:59Z', '2026-09-28T05:00:00Z'],
    displayed: [
      '2026-09-27 05:00:00 UTC+00:00',
      '2026-09-27 04:59:59 UTC+00:00',
      '2026-09-28 05:00:00 UTC+00:00',
    ],
  },
  {
    name: 'preserves different offsets across a daylight-saving transition',
    zone: 'America/New_York',
    label: 'America/New_York',
    dates: ['2030-03-10T00:00:00-05:00', '2030-03-09T23:59:59-05:00', '2030-03-17T00:00:00-04:00'],
    displayed: [
      '2030-03-10 00:00:00 UTC-05:00',
      '2030-03-09 23:59:59 UTC-05:00',
      '2030-03-17 00:00:00 UTC-04:00',
    ],
  },
  {
    name: 'keeps fractional cutoffs and fractional-hour server offsets',
    zone: 'Local',
    label: 'Server local time',
    dates: [
      '2030-01-01T00:00:00.123+05:45',
      '2030-01-01T00:00:00+05:45',
      '2030-01-02T00:00:00.123+05:45',
    ],
    displayed: [
      '2030-01-01 00:00:00.123 UTC+05:45',
      '2030-01-01 00:00:00 UTC+05:45',
      '2030-01-02 00:00:00.123 UTC+05:45',
    ],
  },
])('$name', async ({ zone, label, dates, displayed }) => {
  const post = vi.spyOn(HttpUtil, 'post').mockResolvedValue(
    new Msg(true, '', {
      timeZone: zone,
      renewAt: dates[0],
      validThrough: dates[1],
      nextExpiry: dates[2],
      suggestedExpiryTime: 0,
      suggestedExpiry: '',
      renewals: 1,
      canRenew: true,
      delayedStart: false,
    }),
  );
  try {
    renderWithProviders(<PreviewForm />);
    await screen.findByText(`Renewal preview (panel time zone: ${label})`);
    const times = Array.from(document.querySelectorAll('time'));
    expect(times.map((el) => el.textContent)).toEqual(displayed);
    expect(times.map((el) => el.dateTime)).toEqual(dates);
    expect(times.map((el) => el.dir)).toEqual(['ltr', 'ltr', 'ltr']);
    expect(screen.getByText('Estimated renewals used: 1').textContent).toBe(
      'Estimated renewals used: 1',
    );
    expect(
      screen.getByText('Dates use the panel time zone. Previewing does not renew the client.')
        .textContent,
    ).toBe('Dates use the panel time zone. Previewing does not renew the client.');
  } finally {
    post.mockRestore();
  }
});

it('explains the server timezone and estimated renewal count in Chinese', async () => {
  const i18n = createInstance();
  await i18n.init({
    lng: 'zh-CN',
    resources: { 'zh-CN': { translation: zhCN } },
    interpolation: { escapeValue: false, prefix: '{', suffix: '}' },
  });
  const post = vi.spyOn(HttpUtil, 'post').mockResolvedValue(
    new Msg(true, '', {
      timeZone: 'Local',
      renewAt: '2026-09-27T05:00:00Z',
      validThrough: '2026-09-27T04:59:59Z',
      nextExpiry: '2026-09-28T05:00:00Z',
      suggestedExpiryTime: 0,
      suggestedExpiry: '',
      renewals: 1,
      canRenew: true,
      delayedStart: false,
    }),
  );
  try {
    renderWithProviders(
      <I18nextProvider i18n={i18n}>
        <PreviewForm />
      </I18nextProvider>,
    );
    await screen.findByText('续期预览（面板时区：服务器本地时区）');
    expect(screen.getByText('预计消耗续期次数：1').textContent).toBe('预计消耗续期次数：1');
    expect(screen.getByText('日期按面板时区显示，查看预览不会执行续期。').textContent).toBe(
      '日期按面板时区显示，查看预览不会执行续期。',
    );
  } finally {
    post.mockRestore();
  }
});
