import { useTranslation } from 'react-i18next';
import { AutoComplete, Input, InputNumber, Select, Switch } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { HeaderMapEditor } from '@/components/form';
import { FormField } from '@/components/form/rhf';
import { XHTTP_SESSION_ID_TABLES, XHttpXmuxSchema } from '@/schemas/protocols/stream/xhttp';
import { validateSessionIDLength, validateSessionIDTable } from '@/lib/xray/xhttp-session-id';

const XMUX_DEFAULTS = XHttpXmuxSchema.parse({});

function antdValidatorToRhf(fn: (rule: unknown, value: unknown) => Promise<void>) {
  return async (value: unknown): Promise<true | string> => {
    try {
      await fn(undefined, value);
      return true;
    } catch (e) {
      return (e as Error).message;
    }
  };
}

export default function XhttpForm() {
  const { t } = useTranslation();
  const { control, getValues, setValue } = useFormContext();
  const xhttpMode = useWatch({ control, name: 'streamSettings.xhttpSettings.mode' }) as string | undefined;
  const xhttpObfsMode = !!useWatch({ control, name: 'streamSettings.xhttpSettings.xPaddingObfsMode' });
  const xhttpSessionIDPlacement = useWatch({ control, name: 'streamSettings.xhttpSettings.sessionIDPlacement' }) as string | undefined;
  const xhttpSessionIDTable = useWatch({ control, name: 'streamSettings.xhttpSettings.sessionIDTable' });
  const xhttpSeqPlacement = useWatch({ control, name: 'streamSettings.xhttpSettings.seqPlacement' }) as string | undefined;
  const xhttpUplinkPlacement = useWatch({ control, name: 'streamSettings.xhttpSettings.uplinkDataPlacement' }) as string | undefined;
  const enableXmux = !!useWatch({ control, name: 'streamSettings.xhttpSettings.enableXmux' });

  function onXmuxToggle(checked: boolean) {
    if (!checked) return;
    const existing = getValues('streamSettings.xhttpSettings.xmux');
    const hasValues = existing && typeof existing === 'object' && Object.keys(existing).length > 0;
    if (hasValues) return;
    setValue('streamSettings.xhttpSettings.xmux', { ...XMUX_DEFAULTS });
  }

  return (
    <>
      <FormField name={['streamSettings', 'xhttpSettings', 'host']} label={t('host')}>
        <Input />
      </FormField>
      <FormField name={['streamSettings', 'xhttpSettings', 'path']} label={t('path')}>
        <Input />
      </FormField>
      <FormField name={['streamSettings', 'xhttpSettings', 'mode']} label={t('pages.inbounds.info.mode')}>
        <Select
          style={{ width: '50%' }}
          options={(['auto', 'packet-up', 'stream-up', 'stream-one'] as const).map((m) => ({
            value: m,
            label: m,
          }))}
        />
      </FormField>
      {(xhttpMode === 'packet-up' || xhttpMode === 'auto') && (
        <>
          <FormField
            name={['streamSettings', 'xhttpSettings', 'scMaxEachPostBytes']}
            label={t('pages.inbounds.form.maxUploadSize')}
          >
            <Input />
          </FormField>
          <FormField
            name={['streamSettings', 'xhttpSettings', 'scMaxBufferedPosts']}
            label={t('pages.inbounds.form.maxBufferedUpload')}
          >
            <InputNumber />
          </FormField>
          <FormField
            name={['streamSettings', 'xhttpSettings', 'scMinPostsIntervalMs']}
            label={t('pages.xray.outboundForm.minUploadInterval')}
          >
            <Input placeholder="e.g. 50-150" />
          </FormField>
        </>
      )}
      {xhttpMode === 'stream-up' && (
        <>
          <FormField
            name={['streamSettings', 'xhttpSettings', 'scMaxBufferedPosts']}
            label={t('pages.inbounds.form.maxBufferedUpload')}
          >
            <InputNumber />
          </FormField>
          <FormField
            name={['streamSettings', 'xhttpSettings', 'scStreamUpServerSecs']}
            label={t('pages.inbounds.form.streamUpServer')}
          >
            <Input />
          </FormField>
        </>
      )}
      <FormField
        name={['streamSettings', 'xhttpSettings', 'serverMaxHeaderBytes']}
        label={t('pages.inbounds.form.serverMaxHeaderBytes')}
      >
        <InputNumber min={0} placeholder="0 (default)" />
      </FormField>
      <FormField
        name={['streamSettings', 'xhttpSettings', 'xPaddingBytes']}
        label={t('pages.inbounds.form.paddingBytes')}
      >
        <Input />
      </FormField>
      <FormField
        name={['streamSettings', 'xhttpSettings', 'headers']}
        label={t('pages.inbounds.form.headers')}
      >
        <HeaderMapEditor mode="v1" />
      </FormField>
      <FormField
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
      </FormField>
      <FormField
        name={['streamSettings', 'xhttpSettings', 'xPaddingObfsMode']}
        label={t('pages.inbounds.form.paddingObfsMode')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {xhttpObfsMode && (
        <>
          <FormField
            name={['streamSettings', 'xhttpSettings', 'xPaddingKey']}
            label={t('pages.inbounds.form.paddingKey')}
          >
            <Input placeholder="x_padding" />
          </FormField>
          <FormField
            name={['streamSettings', 'xhttpSettings', 'xPaddingHeader']}
            label={t('pages.inbounds.form.paddingHeader')}
          >
            <Input placeholder="X-Padding" />
          </FormField>
          <FormField
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
          </FormField>
          <FormField
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
          </FormField>
        </>
      )}
      <FormField
        name={['streamSettings', 'xhttpSettings', 'sessionIDPlacement']}
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
      </FormField>
      {xhttpSessionIDPlacement && xhttpSessionIDPlacement !== 'path' && (
        <FormField
          name={['streamSettings', 'xhttpSettings', 'sessionIDKey']}
          label={t('pages.inbounds.form.sessionKey')}
        >
          <Input placeholder="x_session" />
        </FormField>
      )}
      <FormField
        name={['streamSettings', 'xhttpSettings', 'sessionIDTable']}
        label={t('pages.inbounds.form.sessionIDTable')}
        tooltip={t('pages.inbounds.form.sessionIDTableHint')}
        rules={{ validate: antdValidatorToRhf(validateSessionIDTable) }}
      >
        <AutoComplete
          allowClear
          options={XHTTP_SESSION_ID_TABLES.map((v) => ({ value: v }))}
          placeholder="Base62"
        />
      </FormField>
      {!!xhttpSessionIDTable && (
        <FormField
          name={['streamSettings', 'xhttpSettings', 'sessionIDLength']}
          label={t('pages.inbounds.form.sessionIDLength')}
          tooltip={t('pages.inbounds.form.sessionIDLengthHint')}
          rules={{ validate: antdValidatorToRhf(validateSessionIDLength) }}
        >
          <Input placeholder="8-16" />
        </FormField>
      )}
      <FormField
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
      </FormField>
      {xhttpSeqPlacement && xhttpSeqPlacement !== 'path' && (
        <FormField
          name={['streamSettings', 'xhttpSettings', 'seqKey']}
          label={t('pages.inbounds.form.sequenceKey')}
        >
          <Input placeholder="x_seq" />
        </FormField>
      )}
      {xhttpMode === 'packet-up' && (
        <>
          <FormField
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
          </FormField>
          {xhttpUplinkPlacement && xhttpUplinkPlacement !== 'body' && (
            <FormField
              name={['streamSettings', 'xhttpSettings', 'uplinkDataKey']}
              label={t('pages.inbounds.form.uplinkDataKey')}
            >
              <Input placeholder="x_data" />
            </FormField>
          )}
        </>
      )}
      <FormField
        name={['streamSettings', 'xhttpSettings', 'noSSEHeader']}
        label={t('pages.inbounds.form.noSseHeader')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {/* XMUX is the connection-multiplexing layer
          xHTTP uses to fan out parallel requests over
          a small pool of upstream connections. UI-only
          toggle (enableXmux) hides the 6 nested knobs
          when off. */}
      <FormField
        label="XMUX"
        name={['streamSettings', 'xhttpSettings', 'enableXmux']}
        valueProp="checked"
        onAfterChange={(v) => onXmuxToggle(v as boolean)}
      >
        <Switch />
      </FormField>
      {enableXmux && (
        <>
          <FormField
            label={t('pages.xray.outboundForm.maxConcurrency')}
            name={['streamSettings', 'xhttpSettings', 'xmux', 'maxConcurrency']}
          >
            <Input placeholder="16-32" />
          </FormField>
          <FormField
            label={t('pages.xray.outboundForm.maxConnections')}
            name={['streamSettings', 'xhttpSettings', 'xmux', 'maxConnections']}
          >
            <Input placeholder="0" />
          </FormField>
          <FormField
            label={t('pages.xray.outboundForm.maxReuseTimes')}
            name={['streamSettings', 'xhttpSettings', 'xmux', 'cMaxReuseTimes']}
          >
            <Input />
          </FormField>
          <FormField
            label={t('pages.xray.outboundForm.maxRequestTimes')}
            name={['streamSettings', 'xhttpSettings', 'xmux', 'hMaxRequestTimes']}
          >
            <Input placeholder="600-900" />
          </FormField>
          <FormField
            label={t('pages.xray.outboundForm.maxReusableSecs')}
            name={['streamSettings', 'xhttpSettings', 'xmux', 'hMaxReusableSecs']}
          >
            <Input placeholder="1800-3000" />
          </FormField>
          <FormField
            label={t('pages.xray.outboundForm.keepAlivePeriod')}
            name={['streamSettings', 'xhttpSettings', 'xmux', 'hKeepAlivePeriod']}
          >
            <InputNumber min={0} style={{ width: '100%' }} />
          </FormField>
        </>
      )}
    </>
  );
}
