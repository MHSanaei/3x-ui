import { expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';
import { FormProvider, useForm } from 'react-hook-form';

import ClientRenewalFields from '@/pages/clients/ClientRenewalFields';
import { HttpUtil, Msg } from '@/utils';
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
  } finally {
    post.mockRestore();
  }
});
