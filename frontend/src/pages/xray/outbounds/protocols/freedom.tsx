import { useTranslation } from 'react-i18next';
import { AutoComplete, Button, Form, Input, InputNumber, Select, Switch, type FormInstance } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';

import { OutboundDomainStrategies } from '@/schemas/primitives';
import type { OutboundFormValues } from '@/schemas/forms/outbound-form';

export default function FreedomFields({ form }: { form: FormInstance<OutboundFormValues> }) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item label={t('pages.xray.balancer.balancerStrategy')} name={['settings', 'domainStrategy']}>
        <Select
          options={[
            { value: '', label: `(${t('none')})` },
            ...OutboundDomainStrategies.map((s) => ({ value: s, label: s })),
          ]}
        />
      </Form.Item>
      <Form.Item label={t('pages.xray.outboundForm.redirect')} name={['settings', 'redirect']}>
        <Input />
      </Form.Item>
      <Form.Item label={t('pages.xray.tun.userLevel')} name={['settings', 'userLevel']}>
        <InputNumber min={0} style={{ width: '100%' }} />
      </Form.Item>
      <Form.Item label={t('pages.xray.outboundForm.proxyProtocol')} name={['settings', 'proxyProtocol']}>
        <Select
          options={[
            { value: 0, label: `(${t('none')})` },
            { value: 1, label: 'v1' },
            { value: 2, label: 'v2' },
          ]}
        />
      </Form.Item>

      <Form.Item label={t('pages.xray.outboundForm.fragment')} shouldUpdate noStyle>
        {() => {
          const fragment = (form.getFieldValue(['settings', 'fragment']) ?? {}) as {
            packets?: string;
            length?: string;
            interval?: string;
            maxSplit?: string;
          };
          const enabled = !!(fragment.length || fragment.interval || fragment.maxSplit);
          return (
            <>
              <Form.Item label="Fragment">
                <Switch
                  checked={enabled}
                  onChange={(checked) => {
                    form.setFieldValue(
                      ['settings', 'fragment'],
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
              {enabled && (
                <>
                  <Form.Item
                    label={t('pages.settings.subFormats.packets')}
                    name={['settings', 'fragment', 'packets']}
                    rules={[{
                      validator: (_rule, value) => {
                        const str = String(value ?? '').trim();
                        // xray accepts "tlshello" or any packet-number range (#5075)
                        if (str === '' || str === 'tlshello' || /^\d+-\d+$/.test(str)) {
                          return Promise.resolve();
                        }
                        return Promise.reject(new Error('Use "tlshello" or a packet range like 1-3'));
                      },
                    }]}
                  >
                    <AutoComplete
                      options={[
                        { value: 'tlshello', label: 'tlshello' },
                        { value: '1-3', label: '1-3' },
                        { value: '1-5', label: '1-5' },
                      ]}
                      placeholder="tlshello or n-m, e.g. 1-3"
                    />
                  </Form.Item>
                  <Form.Item label={t('pages.settings.subFormats.length')} name={['settings', 'fragment', 'length']}>
                    <Input />
                  </Form.Item>
                  <Form.Item
                    label={t('pages.settings.subFormats.interval')}
                    name={['settings', 'fragment', 'interval']}
                  >
                    <Input />
                  </Form.Item>
                  <Form.Item
                    label={t('pages.settings.subFormats.maxSplit')}
                    name={['settings', 'fragment', 'maxSplit']}
                  >
                    <Input />
                  </Form.Item>
                </>
              )}
            </>
          );
        }}
      </Form.Item>

      <Form.List name={['settings', 'noises']}>
        {(fields, { add, remove }) => (
          <>
            <Form.Item label={t('pages.settings.subFormats.noises')}>
              <Switch
                checked={fields.length > 0}
                onChange={(checked) => {
                  if (checked) {
                    add({
                      type: 'rand',
                      packet: '10-20',
                      delay: '10-16',
                      applyTo: 'ip',
                    });
                  } else {
                    // remove() with no arg is not supported;
                    // walk fields in reverse and drop each.
                    for (let i = fields.length - 1; i >= 0; i--) {
                      remove(fields[i].name);
                    }
                  }
                }}
              />
              {fields.length > 0 && (
                <Button
                  size="small"
                  type="primary"
                  className="ml-8"
                  icon={<PlusOutlined />}
                  onClick={() =>
                    add({
                      type: 'rand',
                      packet: '10-20',
                      delay: '10-16',
                      applyTo: 'ip',
                    })
                  }
                />
              )}
            </Form.Item>
            {fields.map((field, index) => (
              <div key={field.key}>
                <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
                  <div className="item-heading">
                    <span>{t('pages.settings.subFormats.noiseItem', { n: index + 1 })}</span>
                    {fields.length > 1 && (
                      <DeleteOutlined
                        className="danger-icon"
                        onClick={() => remove(field.name)}
                      />
                    )}
                  </div>
                </Form.Item>
                <Form.Item label={t('pages.settings.subFormats.type')} name={[field.name, 'type']}>
                  <Select
                    options={['rand', 'base64', 'str', 'hex'].map((v) => ({
                      value: v,
                      label: v,
                    }))}
                  />
                </Form.Item>
                <Form.Item label={t('pages.settings.subFormats.packet')} name={[field.name, 'packet']}>
                  <Input />
                </Form.Item>
                <Form.Item label={t('pages.settings.subFormats.delayMs')} name={[field.name, 'delay']}>
                  <Input />
                </Form.Item>
                <Form.Item label={t('pages.settings.subFormats.applyTo')} name={[field.name, 'applyTo']}>
                  <Select
                    options={['ip', 'ipv4', 'ipv6'].map((v) => ({
                      value: v,
                      label: v,
                    }))}
                  />
                </Form.Item>
              </div>
            ))}
          </>
        )}
      </Form.List>

      <Form.List name={['settings', 'finalRules']}>
        {(fields, { add, remove }) => (
          <>
            <Form.Item label={t('pages.xray.outboundForm.finalRules')}>
              <Button
                size="small"
                type="primary"
                icon={<PlusOutlined />}
                onClick={() =>
                  add({
                    action: 'allow',
                    network: '',
                    port: '',
                    ip: [],
                    blockDelay: '',
                  })
                }
              />
              <span className="ml-8" style={{ opacity: 0.6 }}>
                {t('pages.xray.outboundForm.overrideXrayPrivateIp')}
              </span>
            </Form.Item>
            {fields.map((field, index) => (
              <div key={field.key}>
                <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
                  <div className="item-heading">
                    <span>{t('pages.xray.outboundForm.ruleN', { n: index + 1 })}</span>
                    <DeleteOutlined
                      className="danger-icon"
                      onClick={() => remove(field.name)}
                    />
                  </div>
                </Form.Item>
                <Form.Item label={t('pages.xray.outboundForm.action')} name={[field.name, 'action']}>
                  <Select
                    options={['allow', 'block'].map((v) => ({
                      value: v,
                      label: v,
                    }))}
                  />
                </Form.Item>
                <Form.Item label={t('pages.inbounds.network')} name={[field.name, 'network']}>
                  <Select
                    allowClear
                    placeholder="(any)"
                    options={['tcp', 'udp', 'tcp,udp'].map((v) => ({
                      value: v,
                      label: v,
                    }))}
                  />
                </Form.Item>
                <Form.Item label={t('pages.inbounds.port')} name={[field.name, 'port']}>
                  <Input placeholder="e.g. 80,443 or 1000-2000" />
                </Form.Item>
                <Form.Item label="IP / CIDR / geoip" name={[field.name, 'ip']}>
                  <Select
                    mode="tags"
                    tokenSeparators={[',', ' ']}
                    placeholder="e.g. 10.0.0.0/8, geoip:private"
                  />
                </Form.Item>
                <Form.Item shouldUpdate noStyle>
                  {() => {
                    const ruleAction = form.getFieldValue([
                      'settings',
                      'finalRules',
                      field.name,
                      'action',
                    ]);
                    if (ruleAction !== 'block') return null;
                    return (
                      <Form.Item
                        label={t('pages.xray.outboundForm.blockDelay')}
                        name={[field.name, 'blockDelay']}
                      >
                        <Input placeholder="optional: 5000-10000" />
                      </Form.Item>
                    );
                  }}
                </Form.Item>
              </div>
            ))}
          </>
        )}
      </Form.List>
    </>
  );
}
