import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import {
  Alert,
  Button,
  Flex,
  Input,
  InputNumber,
  Select,
  Space,
  Spin,
  Table,
  Tag,
  Tooltip,
  Typography,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import dayjs from 'dayjs';

import { HttpUtil } from '@/utils';
import type {
  RealityFinderCommand,
  RealityFinderReport,
  RealityFinderTarget,
} from '@/generated/types';

const JSON_HEADERS = {
  headers: { 'Content-Type': 'application/json' },
} as const;

interface RealityFinderPanelProps {
  inboundPort: number;
  listen: string;
  nodeId: number | null;
  currentTarget: string;
  onPick: (target: string, serverNames: string[]) => void;
}

// A REALITY inbound behind a local listener is reached through 443 by its front proxy.
export function finderDefaultPort(inboundPort: number, listen: string): number {
  const local = /^(127\.|::1$|localhost$|@)/.test(listen.trim());
  return local || !inboundPort ? 443 : inboundPort;
}

// The command lives for a day; keeping it per inbound in this browser tab lets the scanner be
// closed and reopened (to add another target) without making and running a new one.
const finderStoreKey = (inboundPort: number, listen: string, nodeId: number | null) =>
  `realityFinder.command.${nodeId ?? ''}.${inboundPort}.${listen}`;

export function loadFinderCommand(key: string): RealityFinderCommand | null {
  try {
    const cmd = JSON.parse(sessionStorage.getItem(key) ?? 'null') as RealityFinderCommand | null;
    return cmd && cmd.expiresAt * 1000 > Date.now() ? cmd : null;
  } catch {
    return null;
  }
}

function saveFinderCommand(key: string, cmd: RealityFinderCommand | null) {
  try {
    if (cmd) sessionStorage.setItem(key, JSON.stringify(cmd));
    else sessionStorage.removeItem(key);
  } catch {
    // storage blocked: the command just does not outlive the dialog
  }
}

export function finderNotes(row: RealityFinderTarget): string[] {
  const notes: string[] = [];
  if (row.same24) notes.push('same24');
  else if (row.sameAsn) notes.push('sameAsn');
  if (row.cdn) notes.push('cdn');
  if (!row.dnsMatch) notes.push('dns');
  return notes;
}

export default function RealityFinderPanel({
  inboundPort,
  listen,
  nodeId,
  currentTarget,
  onPick,
}: RealityFinderPanelProps) {
  const { t } = useTranslation();
  const [server, setServer] = useState('');
  const [port, setPort] = useState<number>(() => finderDefaultPort(inboundPort, listen));
  const [scope, setScope] = useState('24');
  const [making, setMaking] = useState(false);
  const [error, setError] = useState('');
  const storeKey = finderStoreKey(inboundPort, listen, nodeId);
  const [command, setCommandState] = useState<RealityFinderCommand | null>(() =>
    loadFinderCommand(storeKey),
  );
  const setCommand = (cmd: RealityFinderCommand | null) => {
    setCommandState(cmd);
    saveFinderCommand(storeKey, cmd);
  };
  const [picked, setPicked] = useState<number | null>(null);

  const reports = useQuery({
    queryKey: ['realityFinder', 'reports', command?.token],
    enabled: !!command,
    refetchInterval: 5000,
    queryFn: async () => {
      const msg = await HttpUtil.get<RealityFinderReport[]>(
        `/panel/api/server/realityFinder/command/${command?.token}`,
        undefined,
        { silent: true },
      );
      if (!msg?.success) {
        saveFinderCommand(storeKey, null);
        throw new Error(msg?.msg || 'expired');
      }
      return msg.obj ?? [];
    },
  });
  const list = reports.data ?? [];
  const report = list.find((r) => r.at === picked) ?? list[list.length - 1];

  const makeCommand = async () => {
    setMaking(true);
    setError('');
    try {
      const current = currentTarget.trim().replace(/:\d+$/, '');
      const msg = await HttpUtil.post<RealityFinderCommand>(
        '/panel/api/server/realityFinder/command',
        { server: server.trim(), port, scope, current, nodeId },
        { ...JSON_HEADERS, silent: true },
      );
      if (msg?.success && msg.obj) {
        setCommand(msg.obj);
        setPicked(null);
      } else setError(msg?.msg || t('pages.inbounds.form.finderCommandError'));
    } finally {
      setMaking(false);
    }
  };

  const columns: ColumnsType<RealityFinderTarget> = [
    {
      title: t('pages.inbounds.form.finderScore'),
      dataIndex: 'score',
      key: 'score',
      width: 70,
      render: (v: number) => (
        <Tag color={v >= 70 ? 'success' : v >= 40 ? 'warning' : 'error'}>{v}</Tag>
      ),
    },
    {
      title: t('pages.inbounds.form.target'),
      dataIndex: 'sni',
      key: 'sni',
      width: 210,
      render: (sni: string, row) => (
        <div style={{ lineHeight: 1.25 }}>
          <div
            style={{
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            {sni}
          </div>
          <div style={{ color: '#999', fontSize: 12 }}>{row.ip}</div>
        </div>
      ),
    },
    {
      title: t('pages.inbounds.form.finderHere'),
      dataIndex: 'filtered',
      key: 'filtered',
      width: 100,
      render: (filtered: boolean | null) =>
        filtered === false ? (
          <Tag color="success">{t('pages.inbounds.form.finderOpen')}</Tag>
        ) : filtered ? (
          <Tag color="error">{t('pages.inbounds.form.finderFiltered')}</Tag>
        ) : (
          <Tag>{t('pages.inbounds.form.finderUntested')}</Tag>
        ),
    },
    {
      title: t('pages.inbounds.form.finderNotes'),
      key: 'notes',
      width: 210,
      render: (_, row) => (
        <Flex wrap gap={4}>
          {finderNotes(row).map((n) => (
            <Tag key={n} color={n === 'same24' || n === 'sameAsn' ? 'green' : undefined}>
              {n === 'same24'
                ? t('pages.inbounds.form.finderSame24')
                : n === 'sameAsn'
                  ? t('pages.inbounds.form.finderSameAsn')
                  : n === 'cdn'
                    ? `CDN: ${row.cdn}`
                    : t('pages.inbounds.form.finderDnsElsewhere')}
            </Tag>
          ))}
        </Flex>
      ),
    },
    {
      title: t('pages.inbounds.form.scanLatency'),
      dataIndex: 'ms',
      key: 'ms',
      width: 85,
      render: (v: number) => (v > 0 ? `${v} ms` : '—'),
    },
    {
      title: t('pages.inbounds.form.scanCert'),
      dataIndex: 'issuer',
      key: 'issuer',
      width: 150,
      ellipsis: true,
      render: (issuer: string, row) => (
        <Tooltip title={row.expires}>
          <span>{issuer || '—'}</span>
        </Tooltip>
      ),
    },
    {
      title: '',
      key: 'action',
      width: 72,
      render: (_, row) => (
        <Button type="link" size="small" onClick={() => onPick(`${row.sni}:443`, [row.sni])}>
          {t('pages.inbounds.form.scanUse')}
        </Button>
      ),
    },
  ];

  return (
    <Space orientation="vertical" size="small" style={{ width: '100%' }}>
      <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
        {t('pages.inbounds.form.finderDesc')}
      </Typography.Paragraph>
      <Flex wrap gap={8} align="end">
        <div style={{ flex: '1 1 200px' }}>
          <div>{t('pages.inbounds.form.finderServer')}</div>
          <Input
            aria-label={t('pages.inbounds.form.finderServer')}
            value={server}
            onChange={(e) => setServer(e.target.value)}
            placeholder={t('pages.inbounds.form.finderServerAuto')}
          />
        </div>
        <div style={{ flex: '0 1 140px' }}>
          <Tooltip title={t('pages.inbounds.form.finderPortTip')}>
            <div>{t('pages.inbounds.form.finderPort')}</div>
          </Tooltip>
          <InputNumber
            aria-label={t('pages.inbounds.form.finderPort')}
            min={0}
            max={65535}
            value={port}
            onChange={(v) => setPort(Number(v) || 0)}
            style={{ width: '100%' }}
          />
        </div>
        <div style={{ flex: '0 1 200px' }}>
          <div>{t('pages.inbounds.form.finderScope')}</div>
          <Select
            aria-label={t('pages.inbounds.form.finderScope')}
            value={scope}
            onChange={setScope}
            style={{ width: '100%' }}
            options={[
              { value: '24', label: '/24' },
              { value: '22', label: '/22' },
              { value: '20', label: '/20' },
              {
                value: 'prefix',
                label: t('pages.inbounds.form.finderScopePrefix'),
              },
              { value: 'asn', label: t('pages.inbounds.form.finderScopeAsn') },
            ]}
          />
        </div>
        <Button type="primary" loading={making} onClick={() => void makeCommand()}>
          {t('pages.inbounds.form.finderMake')}
        </Button>
      </Flex>
      {error ? <Alert type="error" showIcon title={error} /> : null}
      {command ? (
        <>
          <Typography.Text type="secondary">
            {t('pages.inbounds.form.finderRunHint', {
              server: command.server,
              time: dayjs.unix(command.expiresAt).format('YYYY-MM-DD HH:mm'),
            })}
          </Typography.Text>
          <div>
            <Typography.Text strong>{t('pages.inbounds.form.finderRunUnix')}</Typography.Text>
            <Typography.Paragraph code copyable style={{ direction: 'ltr', marginBottom: 4 }}>
              {command.unix}
            </Typography.Paragraph>
            <Typography.Text strong>{t('pages.inbounds.form.finderRunWindows')}</Typography.Text>
            <Typography.Paragraph code copyable style={{ direction: 'ltr', marginBottom: 0 }}>
              {command.windows}
            </Typography.Paragraph>
          </div>
        </>
      ) : null}
      {command && !report ? (
        <Space>
          <Spin size="small" />
          <Typography.Text type="secondary">
            {t('pages.inbounds.form.finderWaiting')}
          </Typography.Text>
        </Space>
      ) : null}
      {report ? (
        <>
          {list.length > 1 ? (
            <Select
              aria-label={t('pages.inbounds.form.finderReport')}
              value={report.at}
              onChange={setPicked}
              style={{ width: '100%' }}
              options={list.map((r) => ({
                value: r.at,
                label: `${dayjs.unix(r.at).format('HH:mm')} — AS${r.network.asn} ${r.network.org} (${r.network.ip})`,
              }))}
            />
          ) : (
            <Typography.Text>
              {`AS${report.network.asn} ${report.network.org} (${report.network.ip}) · ${t(
                'pages.inbounds.form.finderTested',
                { n: report.tested },
              )}`}
            </Typography.Text>
          )}
          {report.warning ? (
            <Alert type="warning" showIcon title={t('pages.inbounds.form.finderWarnVpn')} />
          ) : null}
          {report.router ? (
            <Alert type="info" showIcon title={t('pages.inbounds.form.finderRouter')} />
          ) : null}
          {!report.filterTested ? (
            <Alert type="info" showIcon title={t('pages.inbounds.form.finderNoFilter')} />
          ) : null}
          <Table<RealityFinderTarget>
            size="small"
            rowKey="sni"
            columns={columns}
            dataSource={report.targets}
            pagination={false}
            scroll={{ y: 360, x: 900 }}
            locale={{ emptyText: t('pages.inbounds.form.finderNone') }}
          />
        </>
      ) : null}
    </Space>
  );
}
