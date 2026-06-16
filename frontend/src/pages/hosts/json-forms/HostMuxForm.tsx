import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, InputNumber, Select, Switch } from 'antd';

import { parseJsonObject } from './helpers';

// Mirrors the sub-JSON settings Mux editor (enable + concurrency /
// xudpConcurrency / xudpProxyUDP443), serialized to the host's muxParams JSON
// string ('' = disabled). Uses the same responsive horizontal layout as the
// inbound form: label beside the input on desktop, stacked on mobile.
const DEFAULT_MUX = {
  enabled: true,
  concurrency: 8,
  xudpConcurrency: 16,
  xudpProxyUDP443: 'reject',
};

type MuxObject = typeof DEFAULT_MUX;

export default function HostMuxForm({ value = '', onChange }: { value?: string; onChange?: (next: string) => void }) {
  const { t } = useTranslation();
  const [form] = Form.useForm();
  const [initial] = useState<MuxObject | undefined>(() =>
    value ? ({ ...DEFAULT_MUX, ...parseJsonObject(value) } as MuxObject) : undefined,
  );
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const mux = Form.useWatch('mux', form) as MuxObject | undefined;

  useEffect(() => {
    const next = mux ? JSON.stringify(mux) : '';
    if (next !== value) onChangeRef.current?.(next);
  }, [mux, value]);

  const enabled = mux != null;

  return (
    <Form
      form={form}
      component={false}
      colon={false}
      labelCol={{ sm: { span: 8 } }}
      wrapperCol={{ sm: { span: 14 } }}
      labelWrap
      initialValues={{ mux: initial }}
    >
      <Form.Item label={t('pages.settings.mux')}>
        <Switch
          checked={enabled}
          onChange={(v) => form.setFieldValue('mux', v ? { ...DEFAULT_MUX } : undefined)}
        />
      </Form.Item>
      {enabled && (
        <>
          <Form.Item label={t('pages.settings.subFormats.concurrency')} name={['mux', 'concurrency']}>
            <InputNumber min={-1} max={1024} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label={t('pages.settings.subFormats.xudpConcurrency')} name={['mux', 'xudpConcurrency']}>
            <InputNumber min={-1} max={1024} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label={t('pages.settings.subFormats.xudpUdp443')} name={['mux', 'xudpProxyUDP443']}>
            <Select options={['reject', 'allow', 'skip'].map((p) => ({ value: p, label: p }))} />
          </Form.Item>
        </>
      )}
    </Form>
  );
}
