import { useTranslation } from 'react-i18next';
import { AutoComplete, Button, Form, Input, InputNumber, Select, Switch } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { useFieldArray, useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { activateOnKey } from '@/utils/a11y';
import { OutboundDomainStrategies } from '@/schemas/primitives';

interface FragmentValue {
  packets?: string;
  length?: string;
  interval?: string;
  maxSplit?: string;
}

export default function FreedomFields() {
  const { t } = useTranslation();
  const { control, setValue } = useFormContext();

  const fragment = (useWatch({ control, name: 'settings.fragment' }) ?? {}) as FragmentValue;
  const fragmentEnabled = !!(fragment.length || fragment.interval || fragment.maxSplit);

  const {
    fields: noiseFields,
    append: appendNoise,
    remove: removeNoise,
  } = useFieldArray({ control, name: 'settings.noises' });

  const {
    fields: finalRuleFields,
    append: appendFinalRule,
    remove: removeFinalRule,
  } = useFieldArray({ control, name: 'settings.finalRules' });
  const finalRulesValues = (useWatch({ control, name: 'settings.finalRules' }) ?? []) as { action?: string }[];

  return (
    <>
      <FormField label={t('pages.xray.balancer.balancerStrategy')} name={['settings', 'domainStrategy']}>
        <Select
          options={[
            { value: '', label: `(${t('none')})` },
            ...OutboundDomainStrategies.map((s) => ({ value: s, label: s })),
          ]}
        />
      </FormField>
      <FormField label={t('pages.xray.outboundForm.redirect')} name={['settings', 'redirect']}>
        <Input />
      </FormField>
      <FormField label={t('pages.xray.tun.userLevel')} name={['settings', 'userLevel']}>
        <InputNumber min={0} style={{ width: '100%' }} />
      </FormField>
      <FormField label={t('pages.xray.outboundForm.proxyProtocol')} name={['settings', 'proxyProtocol']}>
        <Select
          options={[
            { value: 0, label: `(${t('none')})` },
            { value: 1, label: 'v1' },
            { value: 2, label: 'v2' },
          ]}
        />
      </FormField>

      <Form.Item label="Fragment">
        <Switch
          checked={fragmentEnabled}
          onChange={(checked) => {
            setValue(
              'settings.fragment',
              checked
                ? {
                  packets: 'tlshello',
                  length: '100-200',
                  interval: '10-20',
                  maxSplit: '300-400',
                }
                : { packets: '', length: '', interval: '', maxSplit: '' },
            );
          }}
        />
      </Form.Item>
      {fragmentEnabled && (
        <>
          <FormField
            label={t('pages.settings.subFormats.packets')}
            name={['settings', 'fragment', 'packets']}
            rules={{
              validate: (value) => {
                const str = String(value ?? '').trim();
                /* xray accepts "tlshello" or any packet-number range (#5075) */
                if (str === '' || str === 'tlshello' || /^\d+-\d+$/.test(str)) return true;
                return 'Use "tlshello" or a packet range like 1-3';
              },
            }}
          >
            <AutoComplete
              options={[
                { value: 'tlshello', label: 'tlshello' },
                { value: '1-3', label: '1-3' },
                { value: '1-5', label: '1-5' },
              ]}
              placeholder="tlshello or n-m, e.g. 1-3"
            />
          </FormField>
          <FormField label={t('pages.settings.subFormats.length')} name={['settings', 'fragment', 'length']}>
            <Input />
          </FormField>
          <FormField label={t('pages.settings.subFormats.interval')} name={['settings', 'fragment', 'interval']}>
            <Input />
          </FormField>
          <FormField label={t('pages.settings.subFormats.maxSplit')} name={['settings', 'fragment', 'maxSplit']}>
            <Input />
          </FormField>
        </>
      )}

      <Form.Item label={t('pages.settings.subFormats.noises')}>
        <Switch
          checked={noiseFields.length > 0}
          onChange={(checked) => {
            if (checked) {
              appendNoise({ type: 'rand', packet: '10-20', delay: '10-16', applyTo: 'ip' });
            } else {
              removeNoise();
            }
          }}
        />
        {noiseFields.length > 0 && (
          <Button
            size="small"
            type="primary"
            className="ml-8"
            icon={<PlusOutlined />}
            aria-label={t('add')}
            onClick={() => appendNoise({ type: 'rand', packet: '10-20', delay: '10-16', applyTo: 'ip' })}
          />
        )}
      </Form.Item>
      {noiseFields.map((field, index) => (
        <div key={field.id}>
          <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
            <div className="item-heading">
              <span>{t('pages.settings.subFormats.noiseItem', { n: index + 1 })}</span>
              {noiseFields.length > 1 && (
                <DeleteOutlined
                  className="danger-icon"
                  role="button"
                  tabIndex={0}
                  aria-label={t('remove')}
                  onClick={() => removeNoise(index)}
                  onKeyDown={activateOnKey(() => removeNoise(index))}
                />
              )}
            </div>
          </Form.Item>
          <FormField label={t('pages.settings.subFormats.type')} name={['settings', 'noises', index, 'type']}>
            <Select
              options={['rand', 'base64', 'str', 'hex'].map((v) => ({
                value: v,
                label: v,
              }))}
            />
          </FormField>
          <FormField label={t('pages.settings.subFormats.packet')} name={['settings', 'noises', index, 'packet']}>
            <Input />
          </FormField>
          <FormField label={t('pages.settings.subFormats.delayMs')} name={['settings', 'noises', index, 'delay']}>
            <Input />
          </FormField>
          <FormField label={t('pages.settings.subFormats.applyTo')} name={['settings', 'noises', index, 'applyTo']}>
            <Select
              options={['ip', 'ipv4', 'ipv6'].map((v) => ({
                value: v,
                label: v,
              }))}
            />
          </FormField>
        </div>
      ))}

      <Form.Item label={t('pages.xray.outboundForm.finalRules')}>
        <Button
          size="small"
          type="primary"
          icon={<PlusOutlined />}
          aria-label={t('add')}
          onClick={() => appendFinalRule({ action: 'allow', network: '', port: '', ip: [], blockDelay: '' })}
        />
        <span className="ml-8" style={{ opacity: 0.6 }}>
          {t('pages.xray.outboundForm.overrideXrayPrivateIp')}
        </span>
      </Form.Item>
      {finalRuleFields.map((field, index) => (
        <div key={field.id}>
          <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
            <div className="item-heading">
              <span>{t('pages.xray.outboundForm.ruleN', { n: index + 1 })}</span>
              <DeleteOutlined
                className="danger-icon"
                role="button"
                tabIndex={0}
                aria-label={t('remove')}
                onClick={() => removeFinalRule(index)}
                onKeyDown={activateOnKey(() => removeFinalRule(index))}
              />
            </div>
          </Form.Item>
          <FormField label={t('pages.xray.outboundForm.action')} name={['settings', 'finalRules', index, 'action']}>
            <Select
              options={['allow', 'block'].map((v) => ({
                value: v,
                label: v,
              }))}
            />
          </FormField>
          <FormField label={t('pages.inbounds.network')} name={['settings', 'finalRules', index, 'network']}>
            <Select
              allowClear
              placeholder="(any)"
              options={['tcp', 'udp', 'tcp,udp'].map((v) => ({
                value: v,
                label: v,
              }))}
            />
          </FormField>
          <FormField label={t('pages.inbounds.port')} name={['settings', 'finalRules', index, 'port']}>
            <Input placeholder="e.g. 80,443 or 1000-2000" />
          </FormField>
          <FormField label="IP / CIDR / geoip" name={['settings', 'finalRules', index, 'ip']}>
            <Select
              mode="tags"
              tokenSeparators={[',', ' ']}
              placeholder="e.g. 10.0.0.0/8, geoip:private"
            />
          </FormField>
          {finalRulesValues[index]?.action === 'block' && (
            <FormField
              label={t('pages.xray.outboundForm.blockDelay')}
              name={['settings', 'finalRules', index, 'blockDelay']}
            >
              <Input placeholder="optional: 5000-10000" />
            </FormField>
          )}
        </div>
      ))}
    </>
  );
}
