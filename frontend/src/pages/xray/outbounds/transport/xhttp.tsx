import { useTranslation } from 'react-i18next';
import { AutoComplete, Input, InputNumber, Select, Switch } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { HeaderMapEditor } from '@/components/form';
import { FormField } from '@/components/form/rhf';
import { validateSessionIDLength, validateSessionIDTable } from '@/lib/xray/xhttp-session-id';
import { int32RangeUpper } from '@/lib/xray/stream-wire-normalize';
import { XHTTP_SESSION_ID_TABLES } from '@/schemas/protocols/stream/xhttp';

import { MODE_OPTIONS } from '../outbound-form-constants';

interface XhttpFormProps {
  onXmuxToggle: (checked: boolean) => void;
}

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

const XH = 'streamSettings.xhttpSettings';

export default function XhttpForm({ onXmuxToggle }: XhttpFormProps) {
  const { t } = useTranslation();
  const { control, getValues, setValue } = useFormContext();
  const mode = useWatch({ control, name: `${XH}.mode` }) as string | undefined;
  const obfs = !!useWatch({ control, name: `${XH}.xPaddingObfsMode` });
  const sessionPlacement = useWatch({ control, name: `${XH}.sessionIDPlacement` }) as string | undefined;
  const table = useWatch({ control, name: `${XH}.sessionIDTable` });
  const seqPlacement = useWatch({ control, name: `${XH}.seqPlacement` }) as string | undefined;
  const uplinkDataPlacement = useWatch({ control, name: `${XH}.uplinkDataPlacement` }) as string | undefined;
  const enableXmux = !!useWatch({ control, name: `${XH}.enableXmux` });

  function onXmuxMaxConcurrencyChange(value: unknown) {
    if (int32RangeUpper(value) <= 0) return;
    if (int32RangeUpper(getValues(`${XH}.xmux.maxConnections`)) > 0) {
      setValue(`${XH}.xmux.maxConnections`, 0);
    }
  }

  function onXmuxMaxConnectionsChange(value: unknown) {
    if (int32RangeUpper(value) <= 0) return;
    if (int32RangeUpper(getValues(`${XH}.xmux.maxConcurrency`)) > 0) {
      setValue(`${XH}.xmux.maxConcurrency`, '');
    }
  }

  return (
    <>
      <FormField label={t('host')} name={['streamSettings', 'xhttpSettings', 'host']}>
        <Input />
      </FormField>
      <FormField label={t('path')} name={['streamSettings', 'xhttpSettings', 'path']}>
        <Input />
      </FormField>
      <FormField label={t('pages.inbounds.info.mode')} name={['streamSettings', 'xhttpSettings', 'mode']}>
        <Select options={MODE_OPTIONS} />
      </FormField>
      <FormField
        label={t('pages.inbounds.form.paddingBytes')}
        name={['streamSettings', 'xhttpSettings', 'xPaddingBytes']}
      >
        <Input />
      </FormField>
      <FormField
        label={t('pages.inbounds.form.headers')}
        name={['streamSettings', 'xhttpSettings', 'headers']}
      >
        <HeaderMapEditor mode="v1" />
      </FormField>

      {/* Padding obfs sub-section: gated by a Switch.
          When on, four extra knobs (key/header/placement/
          method) tune how Xray injects random padding to
          disguise the post body shape. */}
      <FormField
        label={t('pages.inbounds.form.paddingObfsMode')}
        name={['streamSettings', 'xhttpSettings', 'xPaddingObfsMode']}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {obfs && (
        <>
          <FormField
            label={t('pages.inbounds.form.paddingKey')}
            name={['streamSettings', 'xhttpSettings', 'xPaddingKey']}
          >
            <Input placeholder="x_padding" />
          </FormField>
          <FormField
            label={t('pages.inbounds.form.paddingHeader')}
            name={['streamSettings', 'xhttpSettings', 'xPaddingHeader']}
          >
            <Input placeholder="X-Padding" />
          </FormField>
          <FormField
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
          </FormField>
          <FormField
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
          </FormField>
        </>
      )}

      <FormField
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
      </FormField>

      {/* Session + sequence + uplinkData placements:
          three orthogonal slots Xray uses to thread
          request metadata through the transport
          (path / header / cookie / query). Key field
          only matters when placement is not 'path'. */}
      <FormField
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
      </FormField>
      {sessionPlacement && sessionPlacement !== 'path' && (
        <FormField
          label={t('pages.inbounds.form.sessionKey')}
          name={['streamSettings', 'xhttpSettings', 'sessionIDKey']}
        >
          <Input placeholder="x_session" />
        </FormField>
      )}
      <FormField
        label={t('pages.inbounds.form.sessionIDTable')}
        tooltip={t('pages.inbounds.form.sessionIDTableHint')}
        name={['streamSettings', 'xhttpSettings', 'sessionIDTable']}
        rules={{ validate: antdValidatorToRhf(validateSessionIDTable) }}
      >
        <AutoComplete
          allowClear
          options={XHTTP_SESSION_ID_TABLES.map((v) => ({ value: v }))}
          placeholder="Base62"
        />
      </FormField>
      {!!table && (
        <FormField
          label={t('pages.inbounds.form.sessionIDLength')}
          tooltip={t('pages.inbounds.form.sessionIDLengthHint')}
          name={['streamSettings', 'xhttpSettings', 'sessionIDLength']}
          rules={{ validate: antdValidatorToRhf(validateSessionIDLength) }}
        >
          <Input placeholder="8-16" />
        </FormField>
      )}
      <FormField
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
      </FormField>
      {seqPlacement && seqPlacement !== 'path' && (
        <FormField
          label={t('pages.inbounds.form.sequenceKey')}
          name={['streamSettings', 'xhttpSettings', 'seqKey']}
        >
          <Input placeholder="x_seq" />
        </FormField>
      )}

      {/* Mode-conditional sub-sections. */}
      {(mode === 'packet-up' || mode === 'auto') && (
        <>
          <FormField
            label={t('pages.xray.outboundForm.minUploadInterval')}
            name={['streamSettings', 'xhttpSettings', 'scMinPostsIntervalMs']}
          >
            <Input placeholder="e.g. 50-150" />
          </FormField>
          <FormField
            label={t('pages.xray.outboundForm.maxUploadSizeBytes')}
            name={['streamSettings', 'xhttpSettings', 'scMaxEachPostBytes']}
          >
            <Input placeholder="1000000" />
          </FormField>
          <FormField
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
          </FormField>
          {uplinkDataPlacement && uplinkDataPlacement !== 'body' && (
            <>
              <FormField
                label={t('pages.inbounds.form.uplinkDataKey')}
                name={['streamSettings', 'xhttpSettings', 'uplinkDataKey']}
              >
                <Input placeholder="x_data" />
              </FormField>
              <FormField
                label={t('pages.xray.outboundForm.uplinkChunkSize')}
                name={['streamSettings', 'xhttpSettings', 'uplinkChunkSize']}
              >
                <InputNumber
                  min={0}
                  placeholder="0 (unlimited)"
                  style={{ width: '100%' }}
                />
              </FormField>
            </>
          )}
        </>
      )}
      {(mode === 'stream-up' || mode === 'stream-one') && (
        <FormField
          label={t('pages.xray.outboundForm.noGrpcHeader')}
          name={['streamSettings', 'xhttpSettings', 'noGRPCHeader']}
          valueProp="checked"
        >
          <Switch />
        </FormField>
      )}

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
            onAfterChange={onXmuxMaxConcurrencyChange}
          >
            <Input placeholder="16-32" />
          </FormField>
          <FormField
            label={t('pages.xray.outboundForm.maxConnections')}
            name={['streamSettings', 'xhttpSettings', 'xmux', 'maxConnections']}
            onAfterChange={onXmuxMaxConnectionsChange}
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
