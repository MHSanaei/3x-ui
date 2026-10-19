import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Divider, Form, Input, Select, Space, Typography } from 'antd';
import { ThunderboltOutlined } from '@ant-design/icons';

import {
  DEFAULT_DOH_ENDPOINT,
  DEFAULT_ECH_DOMAIN,
  dohEndpointOptions,
  echDomainOptions,
  formatDynamicEch,
  isSupportedDohUrl,
  parseDynamicEch,
} from '@/lib/xray/ech-presets';

interface DynamicEchHelperProps {
  /** Current echConfigList, so an existing dynamic pair can be recovered. */
  value?: string;
  /** True when a locally generated ECH keypair is present. */
  hasLocalKeys?: boolean;
  onApply?: (value: string) => void;
  /** Called when the operator accepts dropping CDN-invalid local keys. */
  onClearLocalKeys?: () => void;
  disabled?: boolean;
}

const CUSTOM = '__custom__';

/* Only echConfigList can hold the dynamic `domain+DoH` form; clearing local keys
 * is a separate, explicitly confirmed step because behind a CDN they are dead. */
export default function DynamicEchHelper({
  value = '',
  hasLocalKeys = false,
  onApply,
  onClearLocalKeys,
  disabled = false,
}: DynamicEchHelperProps) {
  const { t } = useTranslation();
  const parsed = parseDynamicEch(value);
  const [domainChoice, setDomainChoice] = useState<string>(
    () => parsed?.domain ?? DEFAULT_ECH_DOMAIN,
  );
  const [domainInput, setDomainInput] = useState<string>(() => parsed?.domain ?? '');
  const [dohChoice, setDohChoice] = useState<string>(() => parsed?.dohUrl ?? DEFAULT_DOH_ENDPOINT);
  const [dohInput, setDohInput] = useState<string>(parsed?.dohUrl ?? '');
  const [preview, setPreview] = useState<string | null>(null);

  const domain = domainChoice === CUSTOM ? domainInput : domainChoice;
  const doh = dohChoice === CUSTOM ? dohInput : dohChoice;
  const built = formatDynamicEch(domain, doh);

  const apply = () => {
    if (!built) return;
    if (hasLocalKeys) setPreview(built);
    else onApply?.(built);
  };

  const pickDomain = (next: string) => {
    setDomainChoice(next);
    setPreview(null);
  };

  const pickDoh = (next: string) => {
    setDohChoice(next);
    setPreview(null);
  };

  const confirmOverwrite = () => {
    if (!built) return;
    onApply?.(built);
    onClearLocalKeys?.();
    setPreview(null);
  };

  return (
    <div data-testid="ech-dynamic-helper" style={{ maxWidth: 640 }}>
      <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 8 }}>
        {t('pages.inbounds.form.echDynamicIntro')}
      </Typography.Paragraph>

      <Form layout="vertical" size="small" colon={false}>
        <Form.Item label={t('pages.inbounds.form.echDynamicDomain')}>
          <Select
            value={domainChoice}
            disabled={disabled}
            showSearch
            onChange={pickDomain}
            options={[
              ...echDomainOptions(),
              { value: CUSTOM, label: t('pages.inbounds.form.addCustomOption') },
            ]}
          />
        </Form.Item>
        {domainChoice === CUSTOM && (
          <Form.Item>
            <Input
              value={domainInput}
              disabled={disabled}
              placeholder={DEFAULT_ECH_DOMAIN}
              onChange={(e) => {
                setDomainInput(e.target.value);
                setPreview(null);
              }}
            />
          </Form.Item>
        )}

        <Form.Item label={t('pages.inbounds.form.echDynamicDoh')}>
          <Select
            value={dohChoice}
            disabled={disabled}
            onChange={pickDoh}
            options={[
              ...dohEndpointOptions(t),
              {
                label: t('pages.inbounds.form.echDynamicCustom'),
                options: [{ value: CUSTOM, label: t('pages.inbounds.form.echDynamicCustom') }],
              },
            ]}
          />
        </Form.Item>
        {dohChoice === CUSTOM && (
          <Form.Item>
            <Input
              value={dohInput}
              disabled={disabled}
              placeholder={DEFAULT_DOH_ENDPOINT}
              status={dohInput && !isSupportedDohUrl(dohInput) ? 'error' : undefined}
              onChange={(e) => {
                setDohInput(e.target.value);
                setPreview(null);
              }}
            />
          </Form.Item>
        )}
        {dohChoice === CUSTOM && dohInput && !isSupportedDohUrl(dohInput) && (
          <Typography.Paragraph type="danger" style={{ fontSize: 12, marginBottom: 8 }}>
            {t('pages.inbounds.form.echDynamicUnsupportedScheme')}
          </Typography.Paragraph>
        )}
      </Form>

      {built ? (
        <Typography.Paragraph
          code
          copyable={{ text: built }}
          data-testid="ech-dynamic-preview"
          style={{ fontSize: 12, marginBottom: 8, wordBreak: 'break-all' }}
        >
          {built}
        </Typography.Paragraph>
      ) : (
        <Typography.Paragraph type="danger" style={{ fontSize: 12, marginBottom: 8 }}>
          {t('pages.inbounds.form.echDynamicInvalid')}
        </Typography.Paragraph>
      )}

      <Button
        type="primary"
        size="small"
        icon={<ThunderboltOutlined />}
        disabled={disabled || !built}
        data-testid="ech-dynamic-apply"
        onClick={apply}
      >
        {t('pages.inbounds.form.echDynamicApply')}
      </Button>

      {preview && (
        <>
          <Divider style={{ margin: '12px 0' }} />
          <Alert
            type="warning"
            showIcon
            data-testid="ech-dynamic-overwrite-warning"
            title={t('pages.inbounds.form.echDynamicOverwriteTitle')}
            description={
              <>
                <div>{t('pages.inbounds.form.echDynamicOverwriteBody')}</div>
                <div data-testid="ech-dynamic-overwrite-value" style={{ marginTop: 4 }}>
                  <Typography.Text code>{preview}</Typography.Text>
                </div>
              </>
            }
            action={
              <Space orientation="vertical">
                <Button size="small" type="primary" onClick={confirmOverwrite}>
                  {t('pages.inbounds.form.echDynamicOverwriteConfirm')}
                </Button>
                <Button size="small" onClick={() => setPreview(null)}>
                  {t('cancel')}
                </Button>
              </Space>
            }
          />
        </>
      )}
    </div>
  );
}
