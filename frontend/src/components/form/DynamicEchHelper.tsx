import { useId, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Divider, Form, Input, Select, Space, Typography } from 'antd';
import { ThunderboltOutlined } from '@ant-design/icons';

import {
  DEFAULT_DOH_ENDPOINT,
  DEFAULT_ECH_DOMAIN,
  DEFAULT_ECH_PAIR,
  dohEndpointOptions,
  echDomainOptions,
  formatDynamicEch,
  isSupportedDohUrl,
  pairPresetOptions,
  parseDynamicEch,
} from '@/lib/xray/ech-presets';

interface DynamicEchHelperProps {
  /** Current echConfigList, so an existing dynamic pair can be recovered. */
  value?: string;
  /** Node SNI, used by the "resolve from the proxied hostname" target. */
  sni?: string;
  /** True when a locally generated ECH keypair is present. */
  hasLocalKeys?: boolean;
  onApply?: (value: string) => void;
  /** Called when the operator accepts dropping CDN-invalid local keys. */
  onClearLocalKeys?: () => void;
  disabled?: boolean;
}

const CUSTOM = '__custom__';
const AUTO = '__auto__';
const ADVANCED = '__advanced__';

/* Only echConfigList can hold the dynamic `domain+DoH` form; clearing local keys
 * is a separate, explicitly confirmed step because behind a CDN they are dead. */
export default function DynamicEchHelper({
  value = '',
  sni = '',
  hasLocalKeys = false,
  onApply,
  onClearLocalKeys,
  disabled = false,
}: DynamicEchHelperProps) {
  const { t } = useTranslation();
  const uid = useId();
  const parsed = parseDynamicEch(value);
  const presetValues = new Set(
    pairPresetOptions(t).flatMap((group) => group.options.map((o) => o.value)),
  );

  const [preset, setPreset] = useState<string>(() => {
    const current = value.trim();
    if (presetValues.has(current)) return current;
    if (!current || !parsed) return DEFAULT_ECH_PAIR;
    return ADVANCED;
  });
  const [domainChoice, setDomainChoice] = useState<string>(
    () => parsed?.domain ?? DEFAULT_ECH_DOMAIN,
  );
  const [domainInput, setDomainInput] = useState<string>(() => parsed?.domain ?? '');
  const [dohChoice, setDohChoice] = useState<string>(() => parsed?.dohUrl ?? DEFAULT_DOH_ENDPOINT);
  const [dohInput, setDohInput] = useState<string>(parsed?.dohUrl ?? '');
  const [preview, setPreview] = useState<string | null>(null);

  const domain =
    domainChoice === CUSTOM ? domainInput : domainChoice === AUTO ? sni.trim() : domainChoice;
  const doh = dohChoice === CUSTOM ? dohInput : dohChoice;
  const advancedBuilt = formatDynamicEch(domain, doh);
  const built = preset === ADVANCED ? advancedBuilt : preset;

  const apply = () => {
    if (!built) return;
    if (hasLocalKeys) setPreview(built);
    else onApply?.(built);
  };

  const pickPreset = (next: string) => {
    setPreset(next);
    setPreview(null);
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
      <div
        style={{ display: 'flex', alignItems: 'center', gap: 6, fontWeight: 600, marginBottom: 4 }}
      >
        <ThunderboltOutlined aria-hidden />
        {t('pages.inbounds.form.echDynamicTitle')}
      </div>
      <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 8 }}>
        {t('pages.inbounds.form.echDynamicIntro')}
      </Typography.Paragraph>
      {value.trim() !== '' && !parsed && (
        <Typography.Paragraph
          type="warning"
          data-testid="ech-existing-not-dynamic"
          style={{ fontSize: 12, marginBottom: 8 }}
        >
          {t('pages.inbounds.form.echExistingNotDynamic')}
        </Typography.Paragraph>
      )}

      {/* component={false}: rendered inside the inbound/host <form>, and a nested
          <form> element is invalid HTML. */}
      <Form layout="vertical" size="small" colon={false} component={false}>
        <Form.Item htmlFor={`${uid}-preset`} label={t('pages.inbounds.form.echPresetLabel')}>
          <Select
            id={`${uid}-preset`}
            value={preset}
            disabled={disabled}
            showSearch={{ optionFilterProp: 'label' }}
            data-testid="ech-preset-select"
            onChange={pickPreset}
            options={[
              ...pairPresetOptions(t),
              {
                label: t('pages.inbounds.form.addCustomOption'),
                options: [{ value: ADVANCED, label: t('pages.inbounds.form.echPresetAdvanced') }],
              },
            ]}
          />
        </Form.Item>

        {preset === ADVANCED && (
          <>
            <Form.Item htmlFor={`${uid}-domain`} label={t('pages.inbounds.form.echDynamicDomain')}>
              <Select
                id={`${uid}-domain`}
                value={domainChoice}
                disabled={disabled}
                showSearch={{ optionFilterProp: 'label' }}
                onChange={pickDomain}
                options={[
                  { value: AUTO, label: t('pages.inbounds.form.echDomainAuto') },
                  ...echDomainOptions(t),
                  { value: CUSTOM, label: t('pages.inbounds.form.addCustomOption') },
                ]}
              />
            </Form.Item>
            {domainChoice === CUSTOM && (
              <Form.Item>
                <Input
                  aria-label={t('pages.inbounds.form.echDynamicDomain')}
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

            <Form.Item htmlFor={`${uid}-doh`} label={t('pages.inbounds.form.echDynamicDoh')}>
              <Select
                id={`${uid}-doh`}
                value={dohChoice}
                disabled={disabled}
                showSearch={{ optionFilterProp: 'label' }}
                onChange={pickDoh}
                options={[
                  ...dohEndpointOptions(t),
                  {
                    label: t('pages.inbounds.form.addCustomOption'),
                    options: [{ value: CUSTOM, label: t('pages.inbounds.form.echDynamicCustom') }],
                  },
                ]}
              />
            </Form.Item>
            {dohChoice === CUSTOM && (
              <Form.Item>
                <Input
                  aria-label={t('pages.inbounds.form.echDynamicDoh')}
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
          </>
        )}

        <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 8 }}>
          {t('pages.inbounds.form.echMainlandAdvisory')}
        </Typography.Paragraph>
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
        icon={<ThunderboltOutlined aria-hidden />}
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
