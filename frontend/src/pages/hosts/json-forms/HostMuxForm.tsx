import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, InputNumber, Select, Switch } from 'antd';

import { parseJsonObject } from './helpers';

// Mirrors the sub-JSON settings Mux editor (enable + concurrency /
// xudpConcurrency / xudpProxyUDP443), serialized to the host's muxParams JSON
// string ('' = disabled). Uses the same responsive horizontal layout as the
// inbound form: label beside the input on desktop, stacked on mobile.
//
// The fields are flat (not a nested `mux` object) and the enable Switch is a
// real `name="enabled"` field — antd then drives its checked state directly, so
// the toggle works. The sub-fields stay registered (hidden when off) so their
// watches fire reliably.
const DEFAULT_MUX = {
  concurrency: 8,
  xudpConcurrency: 16,
  xudpProxyUDP443: 'reject',
};

interface MuxFields {
  enabled: boolean;
  concurrency: number;
  xudpConcurrency: number;
  xudpProxyUDP443: string;
}

export default function HostMuxForm({ value = '', onChange }: { value?: string; onChange?: (next: string) => void }) {
  const { t } = useTranslation();
  const [form] = Form.useForm<MuxFields>();
  const [initial] = useState<MuxFields>(() => {
    const parsed = value ? { ...DEFAULT_MUX, ...parseJsonObject(value) } : DEFAULT_MUX;
    return { ...(parsed as Omit<MuxFields, 'enabled'>), enabled: value !== '' };
  });
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const enabled = Form.useWatch('enabled', form);
  const concurrency = Form.useWatch('concurrency', form);
  const xudpConcurrency = Form.useWatch('xudpConcurrency', form);
  const xudpProxyUDP443 = Form.useWatch('xudpProxyUDP443', form);

  useEffect(() => {
    const next = enabled
      ? JSON.stringify({ enabled: true, concurrency, xudpConcurrency, xudpProxyUDP443 })
      : '';
    if (next !== value) onChangeRef.current?.(next);
  }, [enabled, concurrency, xudpConcurrency, xudpProxyUDP443, value]);

  return (
    <Form
      form={form}
      component={false}
      colon={false}
      labelCol={{ sm: { span: 8 } }}
      wrapperCol={{ sm: { span: 14 } }}
      labelWrap
      initialValues={initial}
    >
      <Form.Item label={t('pages.settings.mux')} name="enabled" valuePropName="checked">
        <Switch />
      </Form.Item>
      <Form.Item label={t('pages.settings.subFormats.concurrency')} name="concurrency" hidden={!enabled}>
        <InputNumber min={-1} max={1024} style={{ width: '100%' }} />
      </Form.Item>
      <Form.Item label={t('pages.settings.subFormats.xudpConcurrency')} name="xudpConcurrency" hidden={!enabled}>
        <InputNumber min={-1} max={1024} style={{ width: '100%' }} />
      </Form.Item>
      <Form.Item label={t('pages.settings.subFormats.xudpUdp443')} name="xudpProxyUDP443" hidden={!enabled}>
        <Select options={['reject', 'allow', 'skip'].map((p) => ({ value: p, label: p }))} />
      </Form.Item>
    </Form>
  );
}
