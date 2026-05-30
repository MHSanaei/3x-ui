import { useTranslation } from 'react-i18next';
import { Form, Input, Switch } from 'antd';

import { HeaderMapEditor } from '@/components/form';

export default function RawForm() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        name={['streamSettings', 'tcpSettings', 'acceptProxyProtocol']}
        label={t('pages.inbounds.form.proxyProtocol')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item label={`HTTP ${t('camouflage')}`}>
        <Form.Item
          noStyle
          shouldUpdate={(prev, curr) =>
            prev.streamSettings?.tcpSettings?.header?.type
            !== curr.streamSettings?.tcpSettings?.header?.type
          }
        >
          {({ getFieldValue, setFieldValue }) => {
            const headerType = getFieldValue(
              ['streamSettings', 'tcpSettings', 'header', 'type'],
            ) as string | undefined;
            return (
              <Switch
                checked={headerType === 'http'}
                onChange={(v) => {
                  setFieldValue(
                    ['streamSettings', 'tcpSettings', 'header'],
                    v
                      ? {
                        type: 'http',
                        request: {
                          version: '1.1',
                          method: 'GET',
                          path: ['/'],
                          headers: {},
                        },
                        response: {
                          version: '1.1',
                          status: '200',
                          reason: 'OK',
                          headers: {},
                        },
                      }
                      : { type: 'none' },
                  );
                }}
              />
            );
          }}
        </Form.Item>
      </Form.Item>
      {/* Per Xray docs (transports/raw.html#httpheaderobject), the
          `request` object is honored only by outbound proxies; the
          inbound listener reads `response`. Showing Host / Path /
          Method / Version / request-headers on the inbound side was
          a regression from this modal's earlier iteration — those
          inputs wrote to the wire but xray-core ignored them. The
          inbound modal now only exposes the response side. */}
      <Form.Item
        noStyle
        shouldUpdate={(prev, curr) =>
          prev.streamSettings?.tcpSettings?.header?.type
          !== curr.streamSettings?.tcpSettings?.header?.type
        }
      >
        {({ getFieldValue }) => {
          const headerType = getFieldValue(
            ['streamSettings', 'tcpSettings', 'header', 'type'],
          ) as string | undefined;
          if (headerType !== 'http') return null;
          return (
            <>
              <Form.Item
                label={t('pages.inbounds.form.requestVersion')}
                name={[
                  'streamSettings', 'tcpSettings', 'header',
                  'request', 'version',
                ]}
              >
                <Input placeholder="1.1" />
              </Form.Item>
              <Form.Item
                label={t('pages.inbounds.form.requestMethod')}
                name={[
                  'streamSettings', 'tcpSettings', 'header',
                  'request', 'method',
                ]}
              >
                <Input placeholder="GET" />
              </Form.Item>
              <Form.Item
                label={t('pages.inbounds.form.requestPath')}
                name={[
                  'streamSettings', 'tcpSettings', 'header',
                  'request', 'path',
                ]}
                getValueProps={(v) => ({ value: Array.isArray(v) ? v.join(',') : v })}
                getValueFromEvent={(e) => {
                  const raw = (e?.target?.value ?? '') as string;
                  const parts = raw.split(',').map((s) => s.trim()).filter(Boolean);
                  return parts.length > 0 ? parts : ['/'];
                }}
              >
                <Input placeholder="/" />
              </Form.Item>
              <Form.Item
                label={t('pages.inbounds.form.requestHeaders')}
                name={[
                  'streamSettings', 'tcpSettings', 'header',
                  'request', 'headers',
                ]}
              >
                <HeaderMapEditor mode="v2" />
              </Form.Item>
              <Form.Item
                label={t('pages.inbounds.form.responseVersion')}
                name={[
                  'streamSettings', 'tcpSettings', 'header',
                  'response', 'version',
                ]}
              >
                <Input placeholder="1.1" />
              </Form.Item>
              <Form.Item
                label={t('pages.inbounds.form.responseStatus')}
                name={[
                  'streamSettings', 'tcpSettings', 'header',
                  'response', 'status',
                ]}
              >
                <Input placeholder="200" />
              </Form.Item>
              <Form.Item
                label={t('pages.inbounds.form.responseReason')}
                name={[
                  'streamSettings', 'tcpSettings', 'header',
                  'response', 'reason',
                ]}
              >
                <Input placeholder="OK" />
              </Form.Item>
              <Form.Item
                label={t('pages.inbounds.form.responseHeaders')}
                name={[
                  'streamSettings', 'tcpSettings', 'header',
                  'response', 'headers',
                ]}
              >
                <HeaderMapEditor mode="v2" />
              </Form.Item>
            </>
          );
        }}
      </Form.Item>
    </>
  );
}
