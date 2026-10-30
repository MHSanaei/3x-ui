import { useCallback, useEffect, useId, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Divider, Form, Space, Switch, Table, Tag, Typography } from 'antd';
import type { MessageInstance } from 'antd/es/message/interface';
import { DeleteOutlined, SearchOutlined, SwapOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { useWatch } from 'react-hook-form';

import { HttpUtil } from '@/utils';
import type { RealityScanResult, RealityWatchEntry, RealityWatchStatus } from '@/generated/types';
import RealityTargetScannerModal from './RealityTargetScannerModal';
import { REALITY_WATCH_MAX_SPARES, addSpare } from './realityWatch';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };
// the server switches after this many failed checks in a row (realityWatchFailLimit)
const FAIL_LIMIT = 3;

interface Props {
  inboundId: number | null;
  onNode: boolean;
  scanRealityCandidates: (targets?: string) => Promise<RealityScanResult[]>;
  message?: MessageInstance;
  // a new inbound has no id yet: its spares go to the form and are saved after the create
  onDraftChange?: (entry: RealityWatchEntry) => void;
}

export default function RealityWatchSection({
  inboundId,
  onNode,
  scanRealityCandidates,
  message,
  onDraftChange,
}: Props) {
  const { t } = useTranslation();
  const uid = useId();
  const [status, setStatus] = useState<RealityWatchStatus | null>(null);
  const [spares, setSpares] = useState<string[]>([]);
  const [auto, setAuto] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState('');
  const [tests, setTests] = useState<Record<string, RealityScanResult>>({});
  const [scannerOpen, setScannerOpen] = useState(false);
  const formTarget = useWatch({ name: 'streamSettings.realitySettings.target' }) as unknown;
  const isDraft = !inboundId;
  const currentTarget = isDraft
    ? typeof formTarget === 'string'
      ? formTarget
      : ''
    : (status?.target ?? '');

  const show = useCallback((st: RealityWatchStatus) => {
    setStatus(st);
    setSpares(st.spares ?? []);
    setAuto(st.auto);
    setDirty(false);
  }, []);

  useEffect(() => {
    if (!inboundId) return;
    void HttpUtil.get<RealityWatchStatus>(
      `/panel/api/realityWatch/status/${inboundId}`,
      undefined,
      {
        silent: true,
      },
    ).then((msg) => {
      if (msg?.success && msg.obj) show(msg.obj);
    });
  }, [inboundId, show]);

  useEffect(() => {
    if (isDraft) onDraftChange?.({ auto, spares });
  }, [isDraft, auto, spares, onDraftChange]);

  const save = async () => {
    setBusy('save');
    try {
      const msg = await HttpUtil.post<RealityWatchStatus>(
        `/panel/api/realityWatch/save/${inboundId}`,
        { auto, spares },
        { ...JSON_HEADERS, silent: true },
      );
      if (msg?.success && msg.obj) {
        show(msg.obj);
        message?.success(t('pages.inbounds.realityWatch.toasts.saved'));
      } else message?.error(msg?.msg || t('pages.inbounds.realityWatch.toasts.failed'));
    } finally {
      setBusy('');
    }
  };

  const test = async (target: string, inBatch = false) => {
    if (!inBatch) setBusy('test:' + target);
    try {
      const msg = await HttpUtil.post<RealityScanResult>(
        '/panel/api/realityWatch/test',
        { target },
        { ...JSON_HEADERS, silent: true },
      );
      if (msg?.success && msg.obj) setTests((prev) => ({ ...prev, [target]: msg.obj! }));
      else message?.error(msg?.msg || t('pages.inbounds.realityWatch.toasts.failed'));
    } finally {
      if (!inBatch) setBusy('');
    }
  };

  const switchTo = async (target: string) => {
    setBusy('switch:' + target);
    try {
      const msg = await HttpUtil.post(
        `/panel/api/realityWatch/switch/${inboundId}`,
        { target },
        { ...JSON_HEADERS, silent: true },
      );
      if (!msg?.success) {
        message?.error(msg?.msg || t('pages.inbounds.realityWatch.toasts.switchFailed'));
        return;
      }
      message?.success(t('pages.inbounds.realityWatch.toasts.switched', { target }));
      const st = await HttpUtil.get<RealityWatchStatus>(
        `/panel/api/realityWatch/status/${inboundId}`,
        undefined,
        { silent: true },
      );
      if (st?.success && st.obj) show(st.obj);
    } finally {
      setBusy('');
    }
  };

  const failing = !!status?.checkedAt && !status.ok;
  const statusTag = !status?.checkedAt ? (
    <Tag>{t('pages.inbounds.realityWatch.notChecked')}</Tag>
  ) : failing ? (
    <Tag color="error">
      {t('pages.inbounds.realityWatch.failing', { fails: status.fails, limit: FAIL_LIMIT })}
    </Tag>
  ) : (
    <Tag color="success">{t('pages.inbounds.realityWatch.working')}</Tag>
  );

  return (
    <>
      <Divider>{t('pages.inbounds.realityWatch.title')}</Divider>
      <Typography.Paragraph type="secondary">
        {t('pages.inbounds.realityWatch.hint', { limit: FAIL_LIMIT })}
        {isDraft && ` ${t('pages.inbounds.realityWatch.savedWithInbound')}`}
      </Typography.Paragraph>
      {onNode && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          title={t('pages.inbounds.realityWatch.onNode')}
        />
      )}
      {!isDraft && (
        <Form.Item label={t('pages.inbounds.realityWatch.status')}>
          <Space wrap>
            {statusTag}
            {status?.checkedAt && status.ok && status.latencyMs > 0 ? (
              <Typography.Text>{status.latencyMs} ms</Typography.Text>
            ) : null}
            {status?.checkedAt ? (
              <Typography.Text type="secondary">
                {dayjs.unix(status.checkedAt).format('YYYY-MM-DD HH:mm')}
              </Typography.Text>
            ) : null}
            {failing && status.reason ? (
              <Typography.Text type="secondary">{status.reason}</Typography.Text>
            ) : null}
          </Space>
        </Form.Item>
      )}
      <Form.Item label={t('pages.inbounds.realityWatch.auto')} htmlFor={`${uid}-auto`}>
        <Switch
          id={`${uid}-auto`}
          checked={auto}
          onChange={(v) => {
            setAuto(v);
            setDirty(true);
          }}
        />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.realityWatch.spares')}
        labelCol={{ span: 24 }}
        wrapperCol={{ span: 24 }}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Table
            size="small"
            rowKey={(r) => r}
            pagination={false}
            dataSource={spares}
            locale={{ emptyText: t('pages.inbounds.realityWatch.noSpares') }}
            columns={[
              { title: t('pages.inbounds.form.target'), render: (_, r: string) => r },
              {
                title: t('pages.inbounds.realityWatch.realTest'),
                width: 190,
                render: (_, r: string) => {
                  const res = tests[r];
                  return res ? (
                    <Tag color={res.feasible ? 'success' : 'error'} title={res.reason}>
                      {res.feasible
                        ? `${t('pages.inbounds.realityWatch.testOk')}${res.latencyMs > 0 ? ` · ${res.latencyMs} ms` : ''}`
                        : t('pages.inbounds.realityWatch.testFailed')}
                    </Tag>
                  ) : (
                    <Button
                      size="small"
                      loading={busy === 'test:' + r || (busy === 'testAll' && !tests[r])}
                      onClick={() => test(r)}
                    >
                      {t('pages.inbounds.realityWatch.test')}
                    </Button>
                  );
                },
              },
              {
                width: 96,
                render: (_, r: string) => (
                  <Space size={4}>
                    {!isDraft && (
                      <Button
                        size="small"
                        icon={<SwapOutlined aria-hidden />}
                        aria-label={t('pages.inbounds.realityWatch.switchNow')}
                        title={t('pages.inbounds.realityWatch.switchNow')}
                        loading={busy === 'switch:' + r}
                        disabled={dirty || onNode}
                        onClick={() => switchTo(r)}
                      />
                    )}
                    <Button
                      size="small"
                      danger
                      icon={<DeleteOutlined aria-hidden />}
                      aria-label={t('delete')}
                      onClick={() => {
                        setSpares(spares.filter((s) => s !== r));
                        setDirty(true);
                      }}
                    />
                  </Space>
                ),
              },
            ]}
          />
          <Space wrap>
            <Button
              icon={<SearchOutlined aria-hidden />}
              disabled={spares.length >= REALITY_WATCH_MAX_SPARES}
              onClick={() => setScannerOpen(true)}
            >
              {t('pages.inbounds.realityWatch.findSpares')}
            </Button>
            <Button
              disabled={!spares.length || busy !== ''}
              loading={busy === 'testAll'}
              onClick={async () => {
                setBusy('testAll');
                setTests({});
                try {
                  for (const r of spares) await test(r, true);
                } finally {
                  setBusy('');
                }
              }}
            >
              {t('pages.inbounds.realityWatch.testAll')}
            </Button>
            {!isDraft && (
              <Button type="primary" disabled={!dirty} loading={busy === 'save'} onClick={save}>
                {t('pages.inbounds.realityWatch.save')}
              </Button>
            )}
          </Space>
        </Space>
      </Form.Item>
      {!!status?.events?.length && (
        <Form.Item label={t('pages.inbounds.realityWatch.history')}>
          <Space direction="vertical" size={2}>
            {[...status.events].reverse().map((ev) => (
              <Typography.Text key={ev.at + ev.to} type={ev.to ? undefined : 'danger'}>
                {dayjs.unix(ev.at).format('MM-DD HH:mm')} ·{' '}
                {ev.to
                  ? t('pages.inbounds.realityWatch.eventSwitched', { from: ev.from, to: ev.to })
                  : t('pages.inbounds.realityWatch.eventNoSpare', { from: ev.from })}
              </Typography.Text>
            ))}
          </Space>
        </Form.Item>
      )}
      <RealityTargetScannerModal
        open={scannerOpen}
        onClose={() => setScannerOpen(false)}
        scanRealityCandidates={scanRealityCandidates}
        onPick={(r) => {
          setSpares((prev) => addSpare(prev, r.target, currentTarget));
          setDirty(true);
        }}
      />
    </>
  );
}
