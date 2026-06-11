import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Select, Switch, type FormInstance } from 'antd';

import { HeaderMapEditor } from '@/components/form';
import type { InboundFormValues } from '@/schemas/forms/inbound-form';
import { XHttpXmuxSchema } from '@/schemas/protocols/stream/xhttp';

const XMUX_DEFAULTS = XHttpXmuxSchema.parse({});

export default function XhttpForm({ form }: { form: FormInstance<InboundFormValues> }) {
  const { t } = useTranslation();
  const xhttpMode = Form.useWatch(['streamSettings', 'xhttpSettings', 'mode'], form);
  const xhttpObfsMode = Form.useWatch(['streamSettings', 'xhttpSettings', 'xPaddingObfsMode'], form) ?? false;
  const xhttpSessionPlacement = Form.useWatch(['streamSettings', 'xhttpSettings', 'sessionPlacement'], form);
  const xhttpSeqPlacement = Form.useWatch(['streamSettings', 'xhttpSettings', 'seqPlacement'], form);
  const xhttpUplinkPlacement = Form.useWatch(['streamSettings', 'xhttpSettings', 'uplinkDataPlacement'], form);

  function onXmuxToggle(checked: boolean) {
    if (!checked) return;
    const existing = form.getFieldValue(['streamSettings', 'xhttpSettings', 'xmux']);
    const hasValues = existing && typeof existing === 'object' && Object.keys(existing).length > 0;
    if (hasValues) return;
    form.setFieldValue(['streamSettings', 'xhttpSettings', 'xmux'], { ...XMUX_DEFAULTS });
  }

  return (
    <>
      <Form.Item name={['streamSettings', 'xhttpSettings', 'host']} label={t('host')}>
        <Input />
      </Form.Item>
      <Form.Item name={['streamSettings', 'xhttpSettings', 'path']} label={t('path')}>
        <Input />
      </Form.Item>
      <Form.Item name={['streamSettings', 'xhttpSettings', 'mode']} label={t('pages.inbounds.info.mode')}>
        <Select
          style={{ width: '50%' }}
          options={(['auto', 'packet-up', 'stream-up', 'stream-one'] as const).map((m) => ({
            value: m,
            label: m,
          }))}
        />
      </Form.Item>
      {xhttpMode === 'packet-up' && (
        <>
          <Form.Item
            name={['streamSettings', 'xhttpSettings', 'scMaxBufferedPosts']}
            label={t('pages.inbounds.form.maxBufferedUpload')}
          >
            <InputNumber />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'xhttpSettings', 'scMaxEachPostBytes']}
            label={t('pages.inbounds.form.maxUploadSize')}
          >
            <Input />
          </Form.Item>
        </>
      )}
      {xhttpMode === 'stream-up' && (
        <Form.Item
          name={['streamSettings', 'xhttpSettings', 'scStreamUpServerSecs']}
          label={t('pages.inbounds.form.streamUpServer')}
        >
          <Input />
        </Form.Item>
      )}
      <Form.Item
        name={['streamSettings', 'xhttpSettings', 'serverMaxHeaderBytes']}
        label={t('pages.inbounds.form.serverMaxHeaderBytes')}
      >
        <InputNumber min={0} placeholder="0 (default)" />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'xhttpSettings', 'xPaddingBytes']}
        label={t('pages.inbounds.form.paddingBytes')}
      >
        <Input />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'xhttpSettings', 'headers']}
        label={t('pages.inbounds.form.headers')}
      >
        <HeaderMapEditor mode="v1" />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'xhttpSettings', 'uplinkHTTPMethod']}
        label={t('pages.inbounds.form.uplinkHttpMethod')}
      >
        <Select
          options={[
            { value: '', label: 'Default (POST)' },
            { value: 'POST', label: 'POST' },
            { value: 'PUT', label: 'PUT' },
            {
              value: 'GET',
              label: 'GET (packet-up only)',
              disabled: xhttpMode !== 'packet-up',
            },
          ]}
        />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'xhttpSettings', 'xPaddingObfsMode']}
        label={t('pages.inbounds.form.paddingObfsMode')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      {xhttpObfsMode && (
        <>
          <Form.Item
            name={['streamSettings', 'xhttpSettings', 'xPaddingKey']}
            label={t('pages.inbounds.form.paddingKey')}
          >
            <Input placeholder="x_padding" />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'xhttpSettings', 'xPaddingHeader']}
            label={t('pages.inbounds.form.paddingHeader')}
          >
            <Input placeholder="X-Padding" />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'xhttpSettings', 'xPaddingPlacement']}
            label={t('pages.inbounds.form.paddingPlacement')}
          >
            <Select
              options={[
                { value: '', label: 'Default (queryInHeader)' },
                { value: 'queryInHeader', label: 'queryInHeader' },
                { value: 'header', label: 'header' },
                { value: 'cookie', label: 'cookie' },
                { value: 'query', label: 'query' },
              ]}
            />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'xhttpSettings', 'xPaddingMethod']}
            label={t('pages.inbounds.form.paddingMethod')}
          >
            <Select
              options={[
                { value: '', label: 'Default (repeat-x)' },
                { value: 'repeat-x', label: 'repeat-x' },
                { value: 'tokenish', label: 'tokenish' },
              ]}
            />
          </Form.Item>
        </>
      )}
      <Form.Item
        name={['streamSettings', 'xhttpSettings', 'sessionPlacement']}
        label={t('pages.inbounds.form.sessionPlacement')}
      >
        <Select
          options={[
            { value: '', label: 'Default (path)' },
            { value: 'path', label: 'path' },
            { value: 'header', label: 'header' },
            { value: 'cookie', label: 'cookie' },
            { value: 'query', label: 'query' },
          ]}
        />
      </Form.Item>
      {xhttpSessionPlacement && xhttpSessionPlacement !== 'path' && (
        <Form.Item
          name={['streamSettings', 'xhttpSettings', 'sessionKey']}
          label={t('pages.inbounds.form.sessionKey')}
        >
          <Input placeholder="x_session" />
        </Form.Item>
      )}
      <Form.Item
        name={['streamSettings', 'xhttpSettings', 'seqPlacement']}
        label={t('pages.inbounds.form.sequencePlacement')}
      >
        <Select
          options={[
            { value: '', label: 'Default (path)' },
            { value: 'path', label: 'path' },
            { value: 'header', label: 'header' },
            { value: 'cookie', label: 'cookie' },
            { value: 'query', label: 'query' },
          ]}
        />
      </Form.Item>
      {xhttpSeqPlacement && xhttpSeqPlacement !== 'path' && (
        <Form.Item
          name={['streamSettings', 'xhttpSettings', 'seqKey']}
          label={t('pages.inbounds.form.sequenceKey')}
        >
          <Input placeholder="x_seq" />
        </Form.Item>
      )}
      {xhttpMode === 'packet-up' && (
        <>
          <Form.Item
            name={['streamSettings', 'xhttpSettings', 'uplinkDataPlacement']}
            label={t('pages.inbounds.form.uplinkDataPlacement')}
          >
            <Select
              options={[
                { value: '', label: 'Default (body)' },
                { value: 'body', label: 'body' },
                { value: 'header', label: 'header' },
                { value: 'cookie', label: 'cookie' },
                { value: 'query', label: 'query' },
              ]}
            />
          </Form.Item>
          {xhttpUplinkPlacement && xhttpUplinkPlacement !== 'body' && (
            <Form.Item
              name={['streamSettings', 'xhttpSettings', 'uplinkDataKey']}
              label={t('pages.inbounds.form.uplinkDataKey')}
            >
              <Input placeholder="x_data" />
            </Form.Item>
          )}
        </>
      )}
      <Form.Item
        name={['streamSettings', 'xhttpSettings', 'noSSEHeader']}
        label={t('pages.inbounds.form.noSseHeader')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      {/* XMUX is the connection-multiplexing layer
          xHTTP uses to fan out parallel requests over
          a small pool of upstream connections. UI-only
          toggle (enableXmux) hides the 6 nested knobs
          when off. */}
      <Form.Item
        label="XMUX"
        name={['streamSettings', 'xhttpSettings', 'enableXmux']}
        valuePropName="checked"
      >
        <Switch onChange={onXmuxToggle} />
      </Form.Item>
      <Form.Item shouldUpdate noStyle>
        {() => {
          if (!form.getFieldValue([
            'streamSettings', 'xhttpSettings', 'enableXmux',
          ])) return null;
          return (
            <>
              <Form.Item
                label={t('pages.xray.outboundForm.maxConcurrency')}
                name={['streamSettings', 'xhttpSettings', 'xmux', 'maxConcurrency']}
              >
                <Input placeholder="16-32" />
              </Form.Item>
              <Form.Item
                label={t('pages.xray.outboundForm.maxConnections')}
                name={['streamSettings', 'xhttpSettings', 'xmux', 'maxConnections']}
              >
                <Input placeholder="0" />
              </Form.Item>
              <Form.Item
                label={t('pages.xray.outboundForm.maxReuseTimes')}
                name={['streamSettings', 'xhttpSettings', 'xmux', 'cMaxReuseTimes']}
              >
                <Input />
              </Form.Item>
              <Form.Item
                label={t('pages.xray.outboundForm.maxRequestTimes')}
                name={['streamSettings', 'xhttpSettings', 'xmux', 'hMaxRequestTimes']}
              >
                <Input placeholder="600-900" />
              </Form.Item>
              <Form.Item
                label={t('pages.xray.outboundForm.maxReusableSecs')}
                name={['streamSettings', 'xhttpSettings', 'xmux', 'hMaxReusableSecs']}
              >
                <Input placeholder="1800-3000" />
              </Form.Item>
              <Form.Item
                label={t('pages.xray.outboundForm.keepAlivePeriod')}
                name={['streamSettings', 'xhttpSettings', 'xmux', 'hKeepAlivePeriod']}
              >
                <InputNumber min={0} style={{ width: '100%' }} />
              </Form.Item>
            </>
          );
        }}
      </Form.Item>
    </>
  );
}
