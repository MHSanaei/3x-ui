import { useTranslation } from 'react-i18next';
import { InputNumber, Select, Switch } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';

import { isMuxAllowed } from '../outbound-form-helpers';

interface MuxFormProps {
  protocol: string;
  network: string;
}

export default function MuxForm({ protocol, network }: MuxFormProps) {
  const { t } = useTranslation();
  const { control } = useFormContext();
  const flow = (useWatch({ control, name: 'settings.flow' }) ?? '') as string;
  const muxEnabled = !!useWatch({ control, name: 'mux.enabled' });
  if (!isMuxAllowed(protocol, flow, network)) return null;
  return (
    <>
      <FormField
        label={t('pages.settings.mux')}
        name={['mux', 'enabled']}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {muxEnabled && (
        <>
          <FormField
            label={t('pages.settings.subFormats.concurrency')}
            name={['mux', 'concurrency']}
          >
            <InputNumber min={-1} max={1024} />
          </FormField>
          <FormField
            label={t('pages.settings.subFormats.xudpConcurrency')}
            name={['mux', 'xudpConcurrency']}
          >
            <InputNumber min={-1} max={1024} />
          </FormField>
          <FormField
            label={t('pages.settings.subFormats.xudpUdp443')}
            name={['mux', 'xudpProxyUDP443']}
          >
            <Select
              options={['reject', 'allow', 'skip'].map((v) => ({
                value: v,
                label: v,
              }))}
            />
          </FormField>
        </>
      )}
    </>
  );
}
