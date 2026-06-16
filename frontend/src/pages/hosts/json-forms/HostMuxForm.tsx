import { useTranslation } from 'react-i18next';
import { InputNumber, Select, Switch } from 'antd';

import { parseJsonObject } from './helpers';

// Mirrors the sub-JSON settings Mux editor: an enable Switch plus concurrency /
// xudpConcurrency / xudpProxyUDP443. Serialized to the host's muxParams JSON
// string ('' = disabled).
const DEFAULT_MUX = {
  enabled: true,
  concurrency: 8,
  xudpConcurrency: 16,
  xudpProxyUDP443: 'reject',
};

type MuxObject = typeof DEFAULT_MUX;

export default function HostMuxForm({ value = '', onChange }: { value?: string; onChange?: (next: string) => void }) {
  const { t } = useTranslation();
  const enabled = value !== '';
  const obj = { ...DEFAULT_MUX, ...(enabled ? parseJsonObject(value) : {}) } as MuxObject;

  const setEnabled = (v: boolean) => onChange?.(v ? JSON.stringify(DEFAULT_MUX) : '');
  const setField = <K extends keyof MuxObject>(key: K, fieldValue: MuxObject[K]) =>
    onChange?.(JSON.stringify({ ...obj, [key]: fieldValue }));

  return (
    <>
      <Switch checked={enabled} onChange={setEnabled} />
      {enabled && (
        <div style={{ display: 'grid', gridTemplateColumns: '160px 1fr', gap: 8, alignItems: 'center', marginTop: 8 }}>
          <span>{t('pages.settings.subFormats.concurrency')}</span>
          <InputNumber
            value={obj.concurrency}
            min={-1}
            max={1024}
            style={{ width: '100%' }}
            onChange={(v) => setField('concurrency', Number(v) || 0)}
          />
          <span>{t('pages.settings.subFormats.xudpConcurrency')}</span>
          <InputNumber
            value={obj.xudpConcurrency}
            min={-1}
            max={1024}
            style={{ width: '100%' }}
            onChange={(v) => setField('xudpConcurrency', Number(v) || 0)}
          />
          <span>{t('pages.settings.subFormats.xudpUdp443')}</span>
          <Select
            value={obj.xudpProxyUDP443}
            style={{ width: '100%' }}
            onChange={(v) => setField('xudpProxyUDP443', v)}
            options={['reject', 'allow', 'skip'].map((p) => ({ value: p, label: p }))}
          />
        </div>
      )}
    </>
  );
}
