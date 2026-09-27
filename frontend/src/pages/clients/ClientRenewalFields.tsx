import { useEffect, useId, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useFormContext, useWatch } from 'react-hook-form';
import { useQuery } from '@tanstack/react-query';
import { Button, Form, InputNumber, Select, Space, Typography } from 'antd';

import { FormField } from '@/components/form/rhf';
import { ClientRenewalPreviewSchema } from '@/generated/zod';
import { HttpUtil } from '@/utils';
import type { ClientFormValues } from '@/schemas/client';

type RenewalFields = Pick<ClientFormValues, 'reset' | 'resetDay' | 'resetWeekday' | 'resetMax'>;
type RenewalMode = 'none' | 'interval' | 'weekly' | 'monthly';

export default function ClientRenewalFields({
  active,
  expiryTime,
  resetCount = 0,
  bulk = false,
  delayedStart = false,
  setExpiry,
}: {
  active: boolean;
  expiryTime: number;
  resetCount?: number;
  bulk?: boolean;
  delayedStart?: boolean;
  setExpiry: (expiry: number) => void;
}) {
  const { t, i18n } = useTranslation();
  const formId = useId();
  const modeId = 'client-renewal-mode-' + formId;
  const { control, setValue } = useFormContext<RenewalFields>();
  const [reset, resetDay, resetWeekday, resetMax] = useWatch({
    control,
    name: ['reset', 'resetDay', 'resetWeekday', 'resetMax'],
  });
  const mode: RenewalMode =
    resetDay > 0 ? 'monthly' : resetWeekday > 0 ? 'weekly' : reset > 0 ? 'interval' : 'none';
  const request = useMemo(
    () => ({
      expiryTime,
      reset: reset || 0,
      resetDay: resetDay || 0,
      resetWeekday: resetWeekday || 0,
      resetMax: resetMax || 0,
      resetCount,
    }),
    [expiryTime, reset, resetDay, resetWeekday, resetMax, resetCount],
  );
  const [debounced, setDebounced] = useState(request);
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(request), 250);
    return () => clearTimeout(timer);
  }, [request]);
  const query = useQuery({
    queryKey: ['clients', 'renewalPreview', debounced],
    enabled: active && mode !== 'none' && request === debounced,
    retry: false,
    queryFn: async () => {
      const msg = await HttpUtil.post('/panel/api/clients/renewalPreview', debounced, {
        headers: { 'Content-Type': 'application/json' },
        silent: true,
      });
      if (!msg?.success) throw new Error(msg?.msg || 'Renewal preview failed');
      return ClientRenewalPreviewSchema.parse(msg.obj);
    },
  });
  const preview = request === debounced ? query.data : undefined;
  const weekdayFormatter = new Intl.DateTimeFormat(i18n.language, {
    weekday: 'long',
    timeZone: 'UTC',
  });
  function changeMode(next: RenewalMode) {
    setValue('reset', next === 'interval' ? Math.max(1, reset || 0) : 0);
    setValue('resetDay', next === 'monthly' ? Math.max(1, resetDay || 0) : 0);
    setValue('resetWeekday', next === 'weekly' ? Math.max(1, resetWeekday || 0) : 0);
  }
  return (
    <>
      <Form.Item label={t('pages.clients.renewMode')} htmlFor={modeId}>
        <Select
          id={modeId}
          value={mode}
          onChange={changeMode}
          options={[
            { value: 'none', label: t('pages.clients.renewModeNone') },
            { value: 'interval', label: t('pages.clients.renewModeInterval') },
            { value: 'weekly', label: t('pages.clients.renewModeWeekly') },
            { value: 'monthly', label: t('pages.clients.renewModeMonthly') },
          ]}
        />
      </Form.Item>
      {mode === 'interval' && (
        <FormField
          name="reset"
          label={bulk ? t('pages.clients.renew') : t('pages.clients.renewDays')}
          tooltip={t('pages.clients.renewDesc')}
          transform={{ output: (v) => Number(v) || 1 }}
        >
          <InputNumber id={'client-renewal-interval-' + formId} min={1} style={{ width: '100%' }} />
        </FormField>
      )}
      {mode === 'monthly' && (
        <FormField
          name="resetDay"
          label={t('pages.clients.renewOnDay')}
          tooltip={t('pages.clients.renewOnDayDesc')}
          transform={{ output: (v) => Number(v) || 1 }}
        >
          <InputNumber
            id={'client-renewal-day-' + formId}
            min={1}
            max={31}
            style={{ width: '100%' }}
          />
        </FormField>
      )}
      {mode === 'weekly' && (
        <FormField name="resetWeekday" label={t('pages.clients.renewWeekday')}>
          <Select
            id={'client-renewal-weekday-' + formId}
            options={Array.from({ length: 7 }, (_, i) => ({
              value: i + 1,
              label: weekdayFormatter.format(new Date(Date.UTC(2026, 0, i + 5))),
            }))}
          />
        </FormField>
      )}
      {mode !== 'none' && (
        <>
          <FormField
            name="resetMax"
            label={t('pages.clients.renewMax')}
            tooltip={t('pages.clients.renewMaxDesc')}
            transform={{ output: (v) => Number(v) || 0 }}
          >
            <InputNumber min={0} style={{ width: '100%' }} />
          </FormField>
          <Typography.Paragraph type="secondary">
            {t('pages.clients.renewScheduleDesc')}
          </Typography.Paragraph>
          {query.isError && request === debounced && (
            <Typography.Paragraph type="warning">
              {t('pages.clients.renewPreviewError')}
            </Typography.Paragraph>
          )}
          {preview && (
            <Space orientation="vertical" size={4} style={{ marginBottom: 16 }}>
              <Typography.Text>
                {t('pages.clients.renewPreview', { zone: preview.timeZone })}
              </Typography.Text>
              {delayedStart || preview.delayedStart ? (
                <Typography.Text type="secondary">
                  {t('pages.clients.renewFirstUse')}
                </Typography.Text>
              ) : expiryTime === 0 ? (
                <>
                  <Typography.Text type="warning">
                    {t('pages.clients.renewNeedsExpiry')}
                  </Typography.Text>
                  {preview.suggestedExpiryTime > 0 && (
                    <Button onClick={() => setExpiry(preview.suggestedExpiryTime)}>
                      {t('pages.clients.renewSetExpiry')}: {preview.suggestedExpiry}
                    </Button>
                  )}
                </>
              ) : (
                <>
                  <Typography.Text>
                    {t('pages.clients.renewAt')}: {preview.renewAt}
                  </Typography.Text>
                  <Typography.Text>
                    {t('pages.clients.renewValidThrough')}: {preview.validThrough}
                  </Typography.Text>
                  {preview.nextExpiry && (
                    <Typography.Text>
                      {t('pages.clients.renewNextExpiry')}: {preview.nextExpiry}
                    </Typography.Text>
                  )}
                  <Typography.Text>
                    {t('pages.clients.renewPeriods', { count: preview.renewals })}
                  </Typography.Text>
                  {!preview.canRenew && (
                    <Typography.Text type="warning">
                      {t('pages.clients.renewUnavailable')}
                    </Typography.Text>
                  )}
                </>
              )}
            </Space>
          )}
        </>
      )}
    </>
  );
}
