import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Input, Modal, Space, Table, Tag, Tooltip, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';

import type { RealityScanResult } from '@/generated/types';

interface RealityTargetScannerModalProps {
  open: boolean;
  onClose: () => void;
  scanRealityCandidates: (targets?: string) => Promise<RealityScanResult[]>;
  onPick: (result: RealityScanResult) => void;
}

export default function RealityTargetScannerModal({
  open,
  onClose,
  scanRealityCandidates,
  onPick,
}: RealityTargetScannerModalProps) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<RealityScanResult[]>([]);
  const scanRef = useRef(scanRealityCandidates);
  scanRef.current = scanRealityCandidates;

  const runScan = useCallback(async (targets?: string) => {
    setLoading(true);
    try {
      setResults(await scanRef.current(targets));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!open) return;
    setResults([]);
    runScan();
  }, [open, runScan]);

  const columns: ColumnsType<RealityScanResult> = [
    {
      title: t('pages.inbounds.form.target'),
      dataIndex: 'target',
      key: 'target',
      width: 200,
      render: (target: string, row) => (
        <Tooltip title={row.ip ? `${target} — ${row.ip}` : target}>
          <div style={{ lineHeight: 1.25 }}>
            <div style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{target}</div>
            {row.ip ? <div style={{ color: '#999', fontSize: 12 }}>{row.ip}</div> : null}
          </div>
        </Tooltip>
      ),
    },
    {
      title: t('pages.inbounds.form.scanStatus'),
      dataIndex: 'feasible',
      key: 'feasible',
      width: 95,
      render: (feasible: boolean, row) =>
        feasible ? (
          <Tag color="success">{t('pages.inbounds.form.scanFeasible')}</Tag>
        ) : (
          <Tooltip title={row.reason}>
            <Tag color="warning">{t('pages.inbounds.form.scanNotFeasible')}</Tag>
          </Tooltip>
        ),
    },
    {
      title: 'TLS',
      dataIndex: 'tlsVersion',
      key: 'tlsVersion',
      width: 60,
      render: (v: string) => v || '—',
    },
    {
      title: 'ALPN',
      dataIndex: 'alpn',
      key: 'alpn',
      width: 75,
      render: (v: string) => v || '—',
    },
    {
      title: t('pages.inbounds.form.scanCurve'),
      dataIndex: 'curveID',
      key: 'curveID',
      width: 130,
      render: (v: string) => v || '—',
    },
    {
      title: t('pages.inbounds.form.scanCert'),
      dataIndex: 'certSubject',
      key: 'certSubject',
      width: 160,
      ellipsis: true,
      render: (_: string, row) =>
        row.certValid ? (
          <Tooltip title={`${row.certSubject} (${row.certIssuer})`}>
            <span>{row.certSubject || '—'}</span>
          </Tooltip>
        ) : (
          <Tag>{t('pages.inbounds.form.scanCertInvalid')}</Tag>
        ),
    },
    {
      title: t('pages.inbounds.form.scanLatency'),
      dataIndex: 'latencyMs',
      key: 'latencyMs',
      width: 85,
      render: (v: number) => (v > 0 ? `${v} ms` : '—'),
    },
    {
      title: '',
      key: 'action',
      width: 64,
      render: (_, row) => (
        <Button
          type="link"
          size="small"
          onClick={() => {
            onPick(row);
            onClose();
          }}
        >
          {t('pages.inbounds.form.scanUse')}
        </Button>
      ),
    },
  ];

  return (
    <Modal
      open={open}
      onCancel={onClose}
      footer={[
        <Button key="rescan" onClick={() => runScan(query.trim() || undefined)} loading={loading}>
          {t('pages.inbounds.form.scanRescan')}
        </Button>,
        <Button key="close" type="primary" onClick={onClose}>
          {t('close')}
        </Button>,
      ]}
      title={t('pages.inbounds.form.scanModalTitle')}
      width={960}
    >
      <Space orientation="vertical" size="small" style={{ width: '100%' }}>
        <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
          {t('pages.inbounds.form.scanModalDesc')}
        </Typography.Paragraph>
        <Input.Search
          allowClear
          enterButton={t('pages.inbounds.form.scan')}
          loading={loading}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onSearch={() => runScan(query.trim() || undefined)}
          placeholder={t('pages.inbounds.form.scanDiscoverPlaceholder')}
        />
        <Table<RealityScanResult>
          size="small"
          rowKey="target"
          loading={loading}
          columns={columns}
          dataSource={results}
          pagination={false}
          scroll={{ y: 360 }}
        />
      </Space>
    </Modal>
  );
}
