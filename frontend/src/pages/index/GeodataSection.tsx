import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Form, Input, Modal, Select, Space, Spin, Typography, message } from 'antd';
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons';

import { HttpUtil } from '@/utils';

interface GeodataAssetRow {
  url: string;
  file: string;
}

interface GeodataSectionProps {
  active: boolean;
  onBusy: (e: { busy: boolean; tip?: string }) => void;
  onClose: () => void;
}

const DEFAULT_CRON = '0 4 * * *';
// Xray resolves `file` inside its asset directory; plain file names only.
const FILE_NAME_PATTERN = /^[A-Za-z0-9._-]+$/;

function fileNameFromUrl(url: string): string {
  try {
    const seg = new URL(url).pathname.split('/').filter(Boolean).pop() || '';
    return FILE_NAME_PATTERN.test(seg) ? seg : '';
  } catch {
    return '';
  }
}

export default function GeodataSection({ active, onBusy, onClose }: GeodataSectionProps) {
  const { t } = useTranslation();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [cron, setCron] = useState(DEFAULT_CRON);
  const [outbound, setOutbound] = useState<string | undefined>(undefined);
  const [rows, setRows] = useState<GeodataAssetRow[]>([]);
  const [outboundTags, setOutboundTags] = useState<string[]>([]);
  const templateRef = useRef<Record<string, unknown> | null>(null);
  const outboundTestUrlRef = useRef('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/', undefined, { silent: true });
      if (!msg?.success || typeof msg.obj !== 'string') return;
      const payload = JSON.parse(msg.obj) as Record<string, unknown>;
      const template = (payload.xraySetting || {}) as Record<string, unknown>;
      templateRef.current = template;
      outboundTestUrlRef.current =
        typeof payload.outboundTestUrl === 'string' ? payload.outboundTestUrl : '';

      const geodata = (template.geodata || {}) as Record<string, unknown>;
      const assets = Array.isArray(geodata.assets) ? geodata.assets : [];
      setRows(
        assets
          .filter((a): a is Record<string, unknown> => !!a && typeof a === 'object')
          .map((a) => ({ url: String(a.url ?? ''), file: String(a.file ?? '') })),
      );
      setCron(typeof geodata.cron === 'string' && geodata.cron ? geodata.cron : DEFAULT_CRON);
      setOutbound(
        typeof geodata.outbound === 'string' && geodata.outbound ? geodata.outbound : undefined,
      );

      // Download outbound candidates: template outbounds + subscription outbounds.
      // Skip blackhole outbounds — routing a download through one just drops it.
      const tags = new Set<string>();
      const outbounds = Array.isArray(template.outbounds) ? template.outbounds : [];
      for (const o of outbounds) {
        if (!o || typeof o !== 'object') continue;
        const rec = o as Record<string, unknown>;
        if (rec.protocol === 'blackhole') continue;
        const tag = rec.tag;
        if (typeof tag === 'string' && tag) tags.add(tag);
      }
      const subTags = Array.isArray(payload.subscriptionOutboundTags)
        ? payload.subscriptionOutboundTags
        : [];
      for (const tag of subTags) {
        if (typeof tag === 'string' && tag) tags.add(tag);
      }
      setOutboundTags([...tags]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (active) load();
  }, [active, load]);

  function setRow(index: number, patch: Partial<GeodataAssetRow>) {
    setRows((prev) => prev.map((r, i) => (i === index ? { ...r, ...patch } : r)));
  }

  function onUrlBlur(index: number) {
    setRows((prev) =>
      prev.map((r, i) => (i === index && !r.file ? { ...r, file: fileNameFromUrl(r.url) } : r)),
    );
  }

  function save() {
    const template = templateRef.current;
    if (!template) return;
    const assets = rows
      .map((r) => ({ url: r.url.trim(), file: r.file.trim() }))
      .filter((r) => r.url || r.file);
    for (const a of assets) {
      // Xray's geodata downloader accepts HTTPS URLs only.
      if (!/^https:\/\/\S+$/i.test(a.url)) {
        messageApi.error(t('pages.index.geodataInvalidUrl'));
        return;
      }
      if (!FILE_NAME_PATTERN.test(a.file)) {
        messageApi.error(t('pages.index.geodataInvalidFile'));
        return;
      }
    }
    const cronValue = cron.trim();
    if (assets.length > 0 && cronValue && cronValue.split(/\s+/).length !== 5) {
      messageApi.error(t('pages.index.geodataInvalidCron'));
      return;
    }

    modal.confirm({
      title: t('pages.index.geodataConfirmTitle'),
      content: t('pages.index.geodataConfirmContent'),
      okText: t('confirm'),
      cancelText: t('cancel'),
      onOk: async () => {
        const next: Record<string, unknown> = { ...template };
        if (assets.length === 0) {
          delete next.geodata;
        } else {
          const geodata: Record<string, unknown> = { assets };
          if (cronValue) geodata.cron = cronValue;
          if (outbound) geodata.outbound = outbound;
          next.geodata = geodata;
        }
        onClose();
        onBusy({ busy: true, tip: t('pages.index.dontRefresh') });
        try {
          const msg = await HttpUtil.post('/panel/api/xray/update', {
            xraySetting: JSON.stringify(next, null, 2),
            outboundTestUrl: outboundTestUrlRef.current,
          });
          if (msg?.success) {
            await HttpUtil.post('/panel/api/server/restartXrayService');
          }
        } finally {
          onBusy({ busy: false });
        }
      },
    });
  }

  return (
    <div>
      {modalContextHolder}
      {messageContextHolder}
      <Spin spinning={loading}>
        <Alert type="info" className="mb-12" title={t('pages.index.geodataHint')} showIcon />
        <Form layout="vertical">
          <Form.Item label={t('pages.index.geodataCron')} style={{ marginBottom: 8 }}>
            <Input
              value={cron}
              placeholder={DEFAULT_CRON}
              onChange={(e) => setCron(e.target.value)}
            />
          </Form.Item>
          <Form.Item label={t('pages.index.geodataOutbound')} style={{ marginBottom: 8 }}>
            <Select
              style={{ width: '100%' }}
              allowClear
              value={outbound}
              onChange={(v) => setOutbound(v)}
              options={outboundTags.map((tag) => ({ label: tag, value: tag }))}
            />
          </Form.Item>
        </Form>
        <Space orientation="vertical" style={{ width: '100%' }} size={8}>
          {rows.length === 0 && (
            <Typography.Text type="secondary">{t('pages.index.geodataEmpty')}</Typography.Text>
          )}
          {rows.map((row, i) => (
            <Space.Compact key={i} style={{ width: '100%' }}>
              <Input
                style={{ width: '60%' }}
                placeholder="https://example.com/geosite_custom.dat"
                value={row.url}
                onChange={(e) => setRow(i, { url: e.target.value })}
                onBlur={() => onUrlBlur(i)}
              />
              <Input
                style={{ width: '40%' }}
                placeholder={t('pages.index.geodataFile')}
                value={row.file}
                onChange={(e) => setRow(i, { file: e.target.value })}
              />
              <Button
                aria-label={t('delete')}
                icon={<DeleteOutlined />}
                onClick={() => setRows((p) => p.filter((_, j) => j !== i))}
              />
            </Space.Compact>
          ))}
          <div className="actions-row">
            <Button
              icon={<PlusOutlined />}
              onClick={() => setRows((p) => [...p, { url: '', file: '' }])}
            >
              {t('pages.index.geodataAddFile')}
            </Button>
            <Button type="primary" onClick={save} disabled={loading || !templateRef.current}>
              {t('pages.index.geodataSaveRestart')}
            </Button>
          </div>
        </Space>
      </Spin>
    </div>
  );
}
