import { useTranslation } from 'react-i18next';
import { Form, InputNumber, Select, Switch, type FormInstance } from 'antd';

import type { OutboundFormValues } from '@/schemas/forms/outbound-form';

import { isMuxAllowed } from '../outbound-form-helpers';

interface MuxFormProps {
  form: FormInstance<OutboundFormValues>;
  protocol: string;
  network: string;
}

export default function MuxForm({ form, protocol, network }: MuxFormProps) {
  const { t } = useTranslation();
  const flow = (form.getFieldValue(['settings', 'flow']) ?? '') as string;
  if (!isMuxAllowed(protocol, flow, network)) return null;
  return (
    <Form.Item shouldUpdate noStyle>
      {() => {
        const muxEnabled = !!form.getFieldValue(['mux', 'enabled']);
        return (
          <>
            <Form.Item
              label={t('pages.settings.mux')}
              name={['mux', 'enabled']}
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
            {muxEnabled && (
              <>
                <Form.Item
                  label={t('pages.settings.subFormats.concurrency')}
                  name={['mux', 'concurrency']}
                >
                  <InputNumber min={-1} max={1024} />
                </Form.Item>
                <Form.Item
                  label={t('pages.settings.subFormats.xudpConcurrency')}
                  name={['mux', 'xudpConcurrency']}
                >
                  <InputNumber min={-1} max={1024} />
                </Form.Item>
                <Form.Item
                  label={t('pages.settings.subFormats.xudpUdp443')}
                  name={['mux', 'xudpProxyUDP443']}
                >
                  <Select
                    options={['reject', 'allow', 'skip'].map((v) => ({
                      value: v,
                      label: v,
                    }))}
                  />
                </Form.Item>
              </>
            )}
          </>
        );
      }}
    </Form.Item>
  );
}
