import { useTranslation } from 'react-i18next';
import { Checkbox, Form, Select, Switch } from 'antd';

import { SNIFFING_OPTION } from '@/schemas/primitives';

export default function SniffingTab({ sniffingEnabled }: { sniffingEnabled: boolean }) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item name={['sniffing', 'enabled']} label={t('enable')} valuePropName="checked">
        <Switch />
      </Form.Item>

      {sniffingEnabled && (
        <>
          <Form.Item name={['sniffing', 'destOverride']} wrapperCol={{ span: 24 }}>
            <Checkbox.Group>
              {Object.entries(SNIFFING_OPTION).map(([key, value]) => (
                <Checkbox key={key} value={value}>{key}</Checkbox>
              ))}
            </Checkbox.Group>
          </Form.Item>

          <Form.Item
            name={['sniffing', 'metadataOnly']}
            label={t('pages.inbounds.sniffingMetadataOnly')}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>

          <Form.Item
            name={['sniffing', 'routeOnly']}
            label={t('pages.inbounds.sniffingRouteOnly')}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>

          <Form.Item
            name={['sniffing', 'ipsExcluded']}
            label={t('pages.inbounds.sniffingIpsExcluded')}
          >
            <Select
              mode="tags"
              tokenSeparators={[',']}
              placeholder="IP/CIDR/geoip:*/ext:*"
              style={{ width: '100%' }}
            />
          </Form.Item>

          <Form.Item
            name={['sniffing', 'domainsExcluded']}
            label={t('pages.inbounds.sniffingDomainsExcluded')}
          >
            <Select
              mode="tags"
              tokenSeparators={[',']}
              placeholder="domain:*/ext:*"
              style={{ width: '100%' }}
            />
          </Form.Item>
        </>
      )}
    </>
  );
}
