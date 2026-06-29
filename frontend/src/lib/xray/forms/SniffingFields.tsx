import { useTranslation } from 'react-i18next';
import { Form, Select, Switch } from 'antd';
import type { FormInstance } from 'antd/es/form';

import { SNIFFING_OPTION } from '@/schemas/primitives';

const DEST_OPTIONS = Object.entries(SNIFFING_OPTION).map(([label, value]) => ({ value, label }));

export interface SniffingFieldsProps {
  // Base path to the sniffing object in the form, e.g. ['sniffing'] (inbound),
  // ['settings', 'reverseSniffing'] (VLESS reverse), ['settings', 'sniffing']
  // (loopback). All sub-fields hang off this path.
  name: (string | number)[];
  form: FormInstance;
  // Label for the enable toggle — Enable / Reverse Sniffing / Sniffing differ
  // per host.
  enableLabel: string;
}

// Shared sniffing form fragment used everywhere the panel edits an xray
// SniffingConfig: the inbound Sniffing tab, VLESS reverse sniffing, and the
// loopback outbound. Renders the enable toggle plus the destOverride /
// metadataOnly / routeOnly / excluded fields when enabled.
export default function SniffingFields({ name, form, enableLabel }: SniffingFieldsProps) {
  const { t } = useTranslation();
  const enabled = Form.useWatch([...name, 'enabled'], form) ?? false;

  return (
    <>
      <Form.Item label={enableLabel} name={[...name, 'enabled']} valuePropName="checked">
        <Switch />
      </Form.Item>

      {enabled && (
        <>
          <Form.Item name={[...name, 'destOverride']} wrapperCol={{ md: { span: 14, offset: 8 } }}>
            <Select
              mode="multiple"
              className="sniffing-options"
              aria-label={t('pages.inbounds.sniffingDestOverride')}
              options={DEST_OPTIONS}
            />
          </Form.Item>
          <Form.Item
            label={t('pages.inbounds.sniffingMetadataOnly')}
            name={[...name, 'metadataOnly']}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item
            label={t('pages.inbounds.sniffingRouteOnly')}
            name={[...name, 'routeOnly']}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item label={t('pages.inbounds.sniffingIpsExcluded')} name={[...name, 'ipsExcluded']}>
            <Select mode="tags" tokenSeparators={[',']} placeholder="IP/CIDR/geoip:*/ext:*" style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label={t('pages.inbounds.sniffingDomainsExcluded')} name={[...name, 'domainsExcluded']}>
            <Select mode="tags" tokenSeparators={[',']} placeholder="domain:*/ext:*" style={{ width: '100%' }} />
          </Form.Item>
        </>
      )}
    </>
  );
}
