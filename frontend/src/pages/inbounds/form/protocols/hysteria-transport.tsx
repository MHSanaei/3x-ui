import { useTranslation } from 'react-i18next';
import { Form, Tabs, type FormInstance } from 'antd';
import { QuicUdpHopForm } from '@/pages/inbounds/form/transport';

export default function HysteriaTransportSettings({
  form,
  port,
}: {
  form: FormInstance;
  port: number;
}) {
  const { t } = useTranslation();

  return (
    <Tabs
      items={[
        {
          key: 'quic',
          label: t('pages.inbounds.form.quicSettings'),
          children: (
            <QuicUdpHopForm basePort={port} form={form} />
          ),
        },
      ]}
    />
  );
}
