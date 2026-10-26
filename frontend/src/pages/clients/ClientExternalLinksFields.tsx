import { Button, Input, Switch, Tooltip, Typography } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { Controller, useFieldArray, useFormContext } from 'react-hook-form';
import { useTranslation } from 'react-i18next';

import { DateTimePicker } from '@/components/form';
import { FormField, rhfZodValidate } from '@/components/form/rhf';
import { resolveExternalLinkExpiry } from '@/lib/clients/external-link';
import {
  ExternalShareLinkValueSchema,
  ExternalSubscriptionValueSchema,
  type ExternalLinksFormValues,
} from '@/schemas/external-link';

export default function ClientExternalLinksFields({
  clientExpiry = 0,
  showFetchStatus = true,
}: {
  clientExpiry?: number;
  showFetchStatus?: boolean;
}) {
  const { t } = useTranslation();
  const { control } = useFormContext<ExternalLinksFormValues>();
  const { fields, append, remove } = useFieldArray({ control, name: 'externalLinks' });

  return (
    <>
      {(['link', 'subscription'] as const).map((kind) => {
        const rows = fields
          .map((field, index) => ({ field, index }))
          .filter(({ field }) => field.kind === kind);
        return (
          <div key={kind} style={{ marginBottom: kind === 'link' ? 24 : 0 }}>
            <Button
              type="primary"
              icon={<PlusOutlined aria-hidden />}
              onClick={() =>
                append({
                  kind,
                  value: '',
                  remark: '',
                  enable: true,
                  expiryTime: 0,
                  namePrefix: '',
                  lastFetchAt: 0,
                  lastFetchError: '',
                })
              }
            >
              {t(
                kind === 'link'
                  ? 'pages.clients.addExternalLink'
                  : 'pages.clients.addExternalSubscription',
              )}
            </Button>
            <div style={{ marginTop: 12 }}>
              {rows.length === 0 ? (
                <Typography.Text type="secondary">
                  {t(
                    kind === 'link'
                      ? 'pages.clients.noExternalLinks'
                      : 'pages.clients.noExternalSubscriptions',
                  )}
                </Typography.Text>
              ) : (
                rows.map(({ field, index }) => (
                  <div key={field.id} className="external-link-card">
                    <div className="external-link-row">
                      <div className="external-link-enable">
                        <FormField
                          name={`externalLinks.${index}.enable`}
                          valueProp="checked"
                          noStyle
                        >
                          <Switch size="small" aria-label={t('enable')} />
                        </FormField>
                        <span>{t('enable')}</span>
                      </div>
                      <FormField
                        name={`externalLinks.${index}.value`}
                        style={{ flex: 1, minWidth: 0, marginBottom: 0 }}
                        rules={{
                          validate: rhfZodValidate(
                            kind === 'link'
                              ? ExternalShareLinkValueSchema
                              : ExternalSubscriptionValueSchema,
                          ),
                        }}
                      >
                        <Input
                          aria-label={
                            kind === 'link'
                              ? 'vless:// · vmess:// · trojan:// · ss:// · hysteria2:// · wireguard://'
                              : 'https://provider.example/sub/…'
                          }
                          placeholder={
                            kind === 'link'
                              ? 'vless:// · vmess:// · trojan:// · ss:// · hysteria2:// · wireguard://'
                              : 'https://provider.example/sub/…'
                          }
                        />
                      </FormField>
                      <Tooltip title={t('delete')}>
                        <Button
                          aria-label={t('delete')}
                          danger
                          icon={<DeleteOutlined aria-hidden />}
                          onClick={() => remove(index)}
                        />
                      </Tooltip>
                    </div>
                    <div
                      className={`external-link-details ${kind === 'link' ? 'two-cols' : 'three-cols'}`}
                    >
                      <FormField name={`externalLinks.${index}.remark`} noStyle>
                        <Input aria-label={t('remark')} placeholder={t('remark')} />
                      </FormField>
                      {kind === 'subscription' && (
                        <FormField name={`externalLinks.${index}.namePrefix`} noStyle>
                          <Input
                            aria-label={t('pages.clients.namePrefix')}
                            placeholder={t('pages.clients.namePrefix')}
                          />
                        </FormField>
                      )}
                      <Controller
                        control={control}
                        name={`externalLinks.${index}.expiryTime`}
                        render={({ field: expiryField }) => {
                          const displayed = resolveExternalLinkExpiry(
                            expiryField.value,
                            clientExpiry,
                          );
                          return (
                            <DateTimePicker
                              aria-label={t('pages.clients.expiryTime')}
                              value={displayed > 0 ? dayjs(displayed) : null}
                              onChange={(value) =>
                                expiryField.onChange(value ? value.valueOf() : 0)
                              }
                              placeholder={t('pages.inbounds.leaveBlankToNeverExpire')}
                              allowClear={Number(expiryField.value) > 0}
                              maxDate={clientExpiry > 0 ? dayjs(clientExpiry) : undefined}
                            />
                          );
                        }}
                      />
                    </div>
                    {kind === 'subscription' && showFetchStatus && (
                      <Typography.Text
                        type={field.lastFetchError ? 'danger' : 'secondary'}
                        className="external-link-fetch-status"
                      >
                        {field.lastFetchError
                          ? `${t('pages.clients.lastFetchError')}: ${field.lastFetchError}`
                          : field.lastFetchAt > 0
                            ? `${t('pages.clients.lastFetchAt')}: ${dayjs(field.lastFetchAt).format('YYYY-MM-DD HH:mm:ss')}`
                            : t('pages.clients.neverFetched')}
                      </Typography.Text>
                    )}
                  </div>
                ))
              )}
            </div>
          </div>
        );
      })}
    </>
  );
}
