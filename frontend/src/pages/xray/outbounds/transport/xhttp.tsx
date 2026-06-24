import { useTranslation } from 'react-i18next';
import { AutoComplete, Form, Input, InputNumber, Select, Switch, type FormInstance } from 'antd';

import { HeaderMapEditor } from '@/components/form';
import { validateSessionIDLength, validateSessionIDTable } from '@/lib/xray/xhttp-session-id';
import type { OutboundFormValues } from '@/schemas/forms/outbound-form';
import { XHTTP_SESSION_ID_TABLES } from '@/schemas/protocols/stream/xhttp';

import { MODE_OPTIONS } from '../outbound-form-constants';

interface XhttpFormProps {
  form: FormInstance<OutboundFormValues>;
  onXmuxToggle: (checked: boolean) => void;
}

export default function XhttpForm({ form, onXmuxToggle }: XhttpFormProps) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        label={t('host')}
        name={['streamSettings', 'xhttpSettings', 'host']}
      >
        <Input />
      </Form.Item>
      <Form.Item
        label={t('path')}
        name={['streamSettings', 'xhttpSettings', 'path']}
      >
        <Input />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.info.mode')}
        name={['streamSettings', 'xhttpSettings', 'mode']}
      >
        <Select options={MODE_OPTIONS} />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.paddingBytes')}
        name={['streamSettings', 'xhttpSettings', 'xPaddingBytes']}
      >
        <Input />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.headers')}
        name={['streamSettings', 'xhttpSettings', 'headers']}
      >
        <HeaderMapEditor mode="v1" />
      </Form.Item>

      {/* Padding obfs sub-section: gated by a Switch.
          When on, four extra knobs (key/header/placement/
          method) tune how Xray injects random padding to
          disguise the post body shape. */}
      <Form.Item
        label={t('pages.inbounds.form.paddingObfsMode')}
        name={['streamSettings', 'xhttpSettings', 'xPaddingObfsMode']}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item shouldUpdate noStyle>
        {() => {
          const obfs = !!form.getFieldValue([
            'streamSettings', 'xhttpSettings', 'xPaddingObfsMode',
          ]);
          if (!obfs) return null;
          return (
            <>
              <Form.Item
                label={t('pages.inbounds.form.paddingKey')}
                name={['streamSettings', 'xhttpSettings', 'xPaddingKey']}
              >
                <Input placeholder="x_padding" />
              </Form.Item>
              <Form.Item
                label={t('pages.inbounds.form.paddingHeader')}
                name={['streamSettings', 'xhttpSettings', 'xPaddingHeader']}
              >
                <Input placeholder="X-Padding" />
              </Form.Item>
              <Form.Item
                label={t('pages.inbounds.form.paddingPlacement')}
                name={['streamSettings', 'xhttpSettings', 'xPaddingPlacement']}
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
                label={t('pages.inbounds.form.paddingMethod')}
                name={['streamSettings', 'xhttpSettings', 'xPaddingMethod']}
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
          );
        }}
      </Form.Item>

      <Form.Item
        noStyle
        shouldUpdate={(prev, curr) =>
          prev?.streamSettings?.xhttpSettings?.mode !==
          curr?.streamSettings?.xhttpSettings?.mode
        }
      >
        {() => {
          const mode = form.getFieldValue([
            'streamSettings', 'xhttpSettings', 'mode',
          ]);
          return (
            <Form.Item
              label={t('pages.inbounds.form.uplinkHttpMethod')}
              name={['streamSettings', 'xhttpSettings', 'uplinkHTTPMethod']}
            >
              <Select
                placeholder="Default (POST)"
                options={[
                  { value: '', label: 'Default (POST)' },
                  { value: 'POST', label: 'POST' },
                  { value: 'PUT', label: 'PUT' },
                  { value: 'GET', label: 'GET (packet-up only)', disabled: mode !== 'packet-up' },
                ]}
              />
            </Form.Item>
          );
        }}
      </Form.Item>

      {/* Session + sequence + uplinkData placements:
          three orthogonal slots Xray uses to thread
          request metadata through the transport
          (path / header / cookie / query). Key field
          only matters when placement is not 'path'. */}
      <Form.Item
        label={t('pages.inbounds.form.sessionPlacement')}
        name={['streamSettings', 'xhttpSettings', 'sessionIDPlacement']}
      >
        <Select
          placeholder="Default (path)"
          options={[
            { value: '', label: 'Default (path)' },
            { value: 'path', label: 'path' },
            { value: 'header', label: 'header' },
            { value: 'cookie', label: 'cookie' },
            { value: 'query', label: 'query' },
          ]}
        />
      </Form.Item>
      <Form.Item shouldUpdate noStyle>
        {() => {
          const placement = form.getFieldValue([
            'streamSettings', 'xhttpSettings', 'sessionIDPlacement',
          ]);
          if (!placement || placement === 'path') return null;
          return (
            <Form.Item
              label={t('pages.inbounds.form.sessionKey')}
              name={['streamSettings', 'xhttpSettings', 'sessionIDKey']}
            >
              <Input placeholder="x_session" />
            </Form.Item>
          );
        }}
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.sessionIDTable')}
        tooltip={t('pages.inbounds.form.sessionIDTableHint')}
        name={['streamSettings', 'xhttpSettings', 'sessionIDTable']}
        rules={[{ validator: validateSessionIDTable }]}
      >
        <AutoComplete
          allowClear
          options={XHTTP_SESSION_ID_TABLES.map((v) => ({ value: v }))}
          placeholder="Base62"
        />
      </Form.Item>
      <Form.Item shouldUpdate noStyle>
        {() => {
          const table = form.getFieldValue([
            'streamSettings', 'xhttpSettings', 'sessionIDTable',
          ]);
          if (!table) return null;
          return (
            <Form.Item
              label={t('pages.inbounds.form.sessionIDLength')}
              tooltip={t('pages.inbounds.form.sessionIDLengthHint')}
              name={['streamSettings', 'xhttpSettings', 'sessionIDLength']}
              rules={[{ validator: validateSessionIDLength }]}
            >
              <Input placeholder="8-16" />
            </Form.Item>
          );
        }}
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.sequencePlacement')}
        name={['streamSettings', 'xhttpSettings', 'seqPlacement']}
      >
        <Select
          placeholder="Default (path)"
          options={[
            { value: '', label: 'Default (path)' },
            { value: 'path', label: 'path' },
            { value: 'header', label: 'header' },
            { value: 'cookie', label: 'cookie' },
            { value: 'query', label: 'query' },
          ]}
        />
      </Form.Item>
      <Form.Item shouldUpdate noStyle>
        {() => {
          const placement = form.getFieldValue([
            'streamSettings', 'xhttpSettings', 'seqPlacement',
          ]);
          if (!placement || placement === 'path') return null;
          return (
            <Form.Item
              label={t('pages.inbounds.form.sequenceKey')}
              name={['streamSettings', 'xhttpSettings', 'seqKey']}
            >
              <Input placeholder="x_seq" />
            </Form.Item>
          );
        }}
      </Form.Item>

      {/* Mode-conditional sub-sections. */}
      <Form.Item shouldUpdate noStyle>
        {() => {
          const mode = form.getFieldValue([
            'streamSettings', 'xhttpSettings', 'mode',
          ]);
          if (mode !== 'packet-up' && mode !== 'auto') return null;
          return (
            <>
              <Form.Item
                label={t('pages.xray.outboundForm.minUploadInterval')}
                name={['streamSettings', 'xhttpSettings', 'scMinPostsIntervalMs']}
              >
                <Input placeholder="e.g. 50-150" />
              </Form.Item>
              <Form.Item
                label={t('pages.xray.outboundForm.maxUploadSizeBytes')}
                name={['streamSettings', 'xhttpSettings', 'scMaxEachPostBytes']}
              >
                <Input placeholder="1000000" />
              </Form.Item>
              <Form.Item
                label={t('pages.inbounds.form.uplinkDataPlacement')}
                name={['streamSettings', 'xhttpSettings', 'uplinkDataPlacement']}
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
              <Form.Item shouldUpdate noStyle>
                {() => {
                  const place = form.getFieldValue([
                    'streamSettings', 'xhttpSettings', 'uplinkDataPlacement',
                  ]);
                  if (!place || place === 'body') return null;
                  return (
                    <>
                      <Form.Item
                        label={t('pages.inbounds.form.uplinkDataKey')}
                        name={['streamSettings', 'xhttpSettings', 'uplinkDataKey']}
                      >
                        <Input placeholder="x_data" />
                      </Form.Item>
                      <Form.Item
                        label={t('pages.xray.outboundForm.uplinkChunkSize')}
                        name={['streamSettings', 'xhttpSettings', 'uplinkChunkSize']}
                      >
                        <InputNumber
                          min={0}
                          placeholder="0 (unlimited)"
                          style={{ width: '100%' }}
                        />
                      </Form.Item>
                    </>
                  );
                }}
              </Form.Item>
            </>
          );
        }}
      </Form.Item>
      <Form.Item shouldUpdate noStyle>
        {() => {
          const mode = form.getFieldValue([
            'streamSettings', 'xhttpSettings', 'mode',
          ]);
          if (mode !== 'stream-up' && mode !== 'stream-one') return null;
          return (
            <Form.Item
              label={t('pages.xray.outboundForm.noGrpcHeader')}
              name={['streamSettings', 'xhttpSettings', 'noGRPCHeader']}
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          );
        }}
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
