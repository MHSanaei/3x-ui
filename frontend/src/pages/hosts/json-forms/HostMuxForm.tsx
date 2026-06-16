import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Col, InputNumber, Row, Select, Switch } from 'antd';

import { parseJsonObject } from './helpers';

// Mirrors the sub-JSON settings Mux editor: an enable Switch plus concurrency /
// xudpConcurrency / xudpProxyUDP443. Serialized to the host's muxParams JSON
// string ('' = disabled). Label/control stack on mobile (xs) and sit side by
// side from sm up.
const DEFAULT_MUX = {
  enabled: true,
  concurrency: 8,
  xudpConcurrency: 16,
  xudpProxyUDP443: 'reject',
};

type MuxObject = typeof DEFAULT_MUX;

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <Row align="middle" gutter={[8, 4]} style={{ marginBottom: 8 }}>
      <Col xs={24} sm={8}>{label}</Col>
      <Col xs={24} sm={16}>{children}</Col>
    </Row>
  );
}

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
        <div style={{ marginTop: 12 }}>
          <Field label={t('pages.settings.subFormats.concurrency')}>
            <InputNumber
              value={obj.concurrency}
              min={-1}
              max={1024}
              style={{ width: '100%' }}
              onChange={(v) => setField('concurrency', Number(v) || 0)}
            />
          </Field>
          <Field label={t('pages.settings.subFormats.xudpConcurrency')}>
            <InputNumber
              value={obj.xudpConcurrency}
              min={-1}
              max={1024}
              style={{ width: '100%' }}
              onChange={(v) => setField('xudpConcurrency', Number(v) || 0)}
            />
          </Field>
          <Field label={t('pages.settings.subFormats.xudpUdp443')}>
            <Select
              value={obj.xudpProxyUDP443}
              style={{ width: '100%' }}
              onChange={(v) => setField('xudpProxyUDP443', v)}
              options={['reject', 'allow', 'skip'].map((p) => ({ value: p, label: p }))}
            />
          </Field>
        </div>
      )}
    </>
  );
}
