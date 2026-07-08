import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Empty, Input, InputNumber, Modal, Select, Space, Switch, Table, Tabs } from 'antd';
import {
  DatabaseOutlined,
  DeleteOutlined,
  ExperimentOutlined,
  MenuOutlined,
  PlusOutlined,
  ProfileOutlined,
  SettingOutlined,
} from '@ant-design/icons';

import { SettingListItem } from '@/components/ui';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from '@/pages/settings/catTabLabel';
import DnsServerModal from './DnsServerModal';
import type { DnsServerValue } from './DnsServerModal';
import DnsPresetsModal from './DnsPresetsModal';
import type { XraySettingsValue, SetTemplate } from '@/hooks/useXraySetting';
import './DnsTab.css';

import { STRATEGIES, DEFAULT_FAKEDNS } from './helpers';
import type { DnsConfig, HostRow, FakednsRow } from './types';
import { useDnsServerColumns, useFakednsColumns } from './useDnsColumns';

interface DnsTabProps {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
}

export default function DnsTab({ templateSettings, setTemplateSettings }: DnsTabProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const [modal, modalContextHolder] = Modal.useModal();
  const [hostsList, setHostsList] = useState<HostRow[]>([]);
  const [serverModalOpen, setServerModalOpen] = useState(false);
  const [editingServer, setEditingServer] = useState<DnsServerValue | null>(null);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [presetsModalOpen, setPresetsModalOpen] = useState(false);

  const dns = (templateSettings?.dns as DnsConfig | undefined) ?? null;
  const dnsEnabled = !!dns;

  const mutate = useCallback(
    (mutator: (next: XraySettingsValue) => void) => {
      setTemplateSettings((prev) => {
        if (!prev) return prev;
        const clone = JSON.parse(JSON.stringify(prev)) as XraySettingsValue;
        mutator(clone);
        return clone;
      });
    },
    [setTemplateSettings],
  );

  function toggleDNS(enabled: boolean) {
    mutate((next) => {
      if (enabled) {
        (next as { dns?: DnsConfig }).dns = {
          tag: 'dns_inbound',
          queryStrategy: 'UseIP',
          disableCache: false,
          disableFallback: false,
          disableFallbackIfMatch: false,
          useSystemHosts: false,
          enableParallelQuery: false,
          serveStale: false,
          serveExpiredTTL: 0,
          hosts: {},
          servers: [],
        };
        next.fakedns = null;
      } else {
        delete next.dns;
        delete next.fakedns;
      }
    });
  }

  useEffect(() => {
    if (!dns) {
      setHostsList([]);
      return;
    }
    const src = dns.hosts || {};
    setHostsList(
      Object.entries(src).map(([domain, val]) => ({
        domain,
        values: Array.isArray(val) ? [...val] : [String(val)],
      })),
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dnsEnabled]);

  function syncHosts(next: HostRow[]) {
    setHostsList(next);
    mutate((tt) => {
      if (!tt.dns) return;
      const obj: Record<string, string | string[]> = {};
      for (const row of next) {
        if (!row.domain) continue;
        const vals = (row.values || []).filter(Boolean);
        if (vals.length === 0) continue;
        obj[row.domain] = vals.length === 1 ? vals[0] : vals;
      }
      if (Object.keys(obj).length > 0) {
        (tt.dns as DnsConfig).hosts = obj;
      } else if ('hosts' in (tt.dns as DnsConfig)) {
        delete (tt.dns as DnsConfig).hosts;
      }
    });
  }

  function setDnsField<K extends keyof DnsConfig>(key: K, value: DnsConfig[K], omit = false) {
    mutate((tt) => {
      if (!tt.dns) return;
      if (omit && (value == null || (typeof value === 'string' && value.trim() === ''))) {
        delete (tt.dns as Record<string, unknown>)[key as string];
      } else {
        (tt.dns as Record<string, unknown>)[key as string] = value;
      }
    });
  }

  const dnsServers = useMemo(() => {
    const list = dns?.servers || [];
    return list.map((server, idx) => ({ key: idx, server }));
  }, [dns?.servers]);

  // Stable callbacks: the column definitions in useDnsServerColumns are
  // memoized, so they must be able to depend on these (see issue #5155)
  const openEditServer = useCallback(
    (idx: number) => {
      setEditingServer((dns?.servers || [])[idx] || null);
      setEditingIndex(idx);
      setServerModalOpen(true);
    },
    [dns?.servers],
  );
  const deleteServer = useCallback(
    (idx: number) => {
      mutate((tt) => {
        const cfg = tt.dns as DnsConfig | undefined;
        if (cfg?.servers) cfg.servers.splice(idx, 1);
      });
    },
    [mutate],
  );

  const dnsColumns = useDnsServerColumns({ openEditServer, deleteServer });

  function openAddServer() {
    setEditingServer(null);
    setEditingIndex(null);
    setServerModalOpen(true);
  }
  function onServerConfirm(value: DnsServerValue) {
    mutate((tt) => {
      if (!tt.dns) return;
      const cfg = tt.dns as DnsConfig;
      if (!Array.isArray(cfg.servers)) cfg.servers = [];
      if (editingIndex == null) cfg.servers.push(value);
      else cfg.servers[editingIndex] = value;
    });
    setServerModalOpen(false);
  }
  function clearAllServers() {
    modal.confirm({
      title: t('pages.xray.dns.clearAllTitle'),
      content: t('pages.xray.dns.clearAllConfirm'),
      okText: t('delete'),
      okButtonProps: { danger: true },
      cancelText: t('cancel'),
      onOk: () => mutate((tt) => {
        if (tt.dns) (tt.dns as DnsConfig).servers = [];
      }),
    });
  }
  function onPresetInstall(servers: string[]) {
    mutate((tt) => {
      if (tt.dns) (tt.dns as DnsConfig).servers = servers;
    });
    setPresetsModalOpen(false);
  }

  const fakeDnsList = useMemo<{ key: number; ipPool: string; poolSize: number }[]>(() => {
    const list = Array.isArray(templateSettings?.fakedns)
      ? (templateSettings?.fakedns as FakednsRow[])
      : [];
    return list.map((entry, idx) => ({ key: idx, ...entry }));
  }, [templateSettings?.fakedns]);

  const deleteFakedns = useCallback(
    (idx: number) => {
      mutate((tt) => {
        const list = tt.fakedns as FakednsRow[] | undefined;
        if (!list) return;
        list.splice(idx, 1);
        if (list.length === 0) tt.fakedns = null;
      });
    },
    [mutate],
  );
  const updateFakednsField = useCallback(
    (idx: number, field: 'ipPool' | 'poolSize', value: string | number) => {
      mutate((tt) => {
        const list = tt.fakedns as FakednsRow[] | undefined;
        if (!list?.[idx]) return;
        (list[idx] as unknown as Record<string, unknown>)[field] = value;
      });
    },
    [mutate],
  );

  const fakednsColumns = useFakednsColumns({ deleteFakedns, updateFakednsField });

  function addFakedns() {
    mutate((tt) => {
      if (!Array.isArray(tt.fakedns)) tt.fakedns = [];
      (tt.fakedns as FakednsRow[]).push(DEFAULT_FAKEDNS());
    });
  }

  const items = useMemo(() => {
    const out = [
      {
        key: '1',
        label: catTabLabel(<SettingOutlined />, t('pages.xray.generalConfigs'), isMobile),
        children: (
          <>
            <SettingListItem
              paddings="small"
              title={t('pages.xray.dns.enable')}
              description={t('pages.xray.dns.enableDesc')}
              control={<Switch checked={dnsEnabled} onChange={toggleDNS} />}
            />
            {dnsEnabled && (
              <>
                <Alert
                  type="warning"
                  showIcon
                  message={t('pages.xray.dns.dnsLeakWarning')}
                  style={{ marginBottom: 12 }}
                />
                <SettingListItem
                  paddings="small"
                  title={t('pages.xray.dns.tag')}
                  description={t('pages.xray.dns.tagDesc')}
                  control={
                    <Input
                      value={dns?.tag ?? 'dns_inbound'}
                      onChange={(e) => setDnsField('tag', e.target.value)}
                    />
                  }
                />
                <SettingListItem
                  paddings="small"
                  title={t('pages.xray.dns.clientIp')}
                  description={t('pages.xray.dns.clientIpDesc')}
                  control={
                    <Input
                      value={dns?.clientIp ?? ''}
                      onChange={(e) => setDnsField('clientIp', e.target.value, true)}
                    />
                  }
                />
                <SettingListItem
                  paddings="small"
                  title={t('pages.xray.dns.strategy')}
                  description={t('pages.xray.dns.strategyDesc')}
                  control={
                    <Select
                      value={dns?.queryStrategy ?? 'UseIP'}
                      style={{ width: '100%' }}
                      options={STRATEGIES.map((s) => ({ value: s, label: s }))}
                      onChange={(v) => setDnsField('queryStrategy', v)}
                    />
                  }
                />
                {(
                  [
                    ['disableCache', 'pages.xray.dns.disableCache', 'pages.xray.dns.disableCacheDesc'],
                    ['disableFallback', 'pages.xray.dns.disableFallback', 'pages.xray.dns.disableFallbackDesc'],
                    ['disableFallbackIfMatch', 'pages.xray.dns.disableFallbackIfMatch', 'pages.xray.dns.disableFallbackIfMatchDesc'],
                    ['enableParallelQuery', 'pages.xray.dns.enableParallelQuery', 'pages.xray.dns.enableParallelQueryDesc'],
                    ['useSystemHosts', 'pages.xray.dns.useSystemHosts', 'pages.xray.dns.useSystemHostsDesc'],
                    ['serveStale', 'pages.xray.dns.serveStale', 'pages.xray.dns.serveStaleDesc'],
                  ] as const
                ).map(([field, titleKey, descKey]) => (
                  <SettingListItem
                    key={field}
                    paddings="small"
                    title={t(titleKey)}
                    description={t(descKey)}
                    control={
                      <Switch
                        checked={!!dns?.[field]}
                        onChange={(v) => setDnsField(field as keyof DnsConfig, v as never)}
                      />
                    }
                  />
                ))}
                <SettingListItem
                  paddings="small"
                  title={t('pages.xray.dns.serveExpiredTTL')}
                  description={t('pages.xray.dns.serveExpiredTTLDesc')}
                  control={
                    <InputNumber
                      value={dns?.serveExpiredTTL ?? 0}
                      min={0}
                      step={60}
                      style={{ width: '100%' }}
                      onChange={(v) => setDnsField('serveExpiredTTL', Number(v) || 0)}
                    />
                  }
                />
              </>
            )}
          </>
        ),
      },
    ];

    if (dnsEnabled) {
      out.push({
        key: 'hosts',
        label: catTabLabel(<ProfileOutlined />, t('pages.xray.dns.hosts'), isMobile),
        children: hostsList.length === 0 ? (
          <Empty description={t('pages.xray.dns.hostsEmpty')}>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => syncHosts([...hostsList, { domain: '', values: [] }])}>
              {t('pages.xray.dns.hostsAdd')}
            </Button>
          </Empty>
        ) : (
          <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => syncHosts([...hostsList, { domain: '', values: [] }])}>
              {t('pages.xray.dns.hostsAdd')}
            </Button>
            {hostsList.map((row, idx) => (
              <div key={`h${idx}`} className="hosts-row">
                <Input
                  value={row.domain}
                  aria-label={t('pages.xray.dns.hostsDomain')}
                  placeholder={t('pages.xray.dns.hostsDomain')}
                  style={{ flex: '1 1 220px' }}
                  onChange={(e) => {
                    const next = hostsList.map((r, i) => (i === idx ? { ...r, domain: e.target.value } : r));
                    syncHosts(next);
                  }}
                />
                <Select
                  mode="tags"
                  value={row.values}
                  aria-label={t('pages.xray.dns.hostsValues')}
                  placeholder={t('pages.xray.dns.hostsValues')}
                  style={{ flex: '2 1 320px' }}
                  tokenSeparators={[',', ' ']}
                  onChange={(values) => {
                    const next = hostsList.map((r, i) => (i === idx ? { ...r, values } : r));
                    syncHosts(next);
                  }}
                />
                <Button danger aria-label={t('delete')} icon={<DeleteOutlined />} onClick={() => syncHosts(hostsList.filter((_, i) => i !== idx))} />
              </div>
            ))}
          </Space>
        ),
      });

      out.push({
        key: '2',
        label: catTabLabel(<DatabaseOutlined />, 'DNS', isMobile),
        children: dnsServers.length === 0 ? (
          <Empty description={t('emptyDnsDesc')}>
            <Space>
              <Button type="primary" icon={<PlusOutlined />} onClick={openAddServer}>
                {t('pages.xray.dns.add')}
              </Button>
              <Button icon={<MenuOutlined />} onClick={() => setPresetsModalOpen(true)}>
                {t('pages.xray.dns.usePreset')}
              </Button>
            </Space>
          </Empty>
        ) : (
          <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
            <Space wrap>
              <Button type="primary" icon={<PlusOutlined />} onClick={openAddServer}>
                {t('pages.xray.dns.add')}
              </Button>
              <Button icon={<MenuOutlined />} onClick={() => setPresetsModalOpen(true)}>
                {t('pages.xray.dns.usePreset')}
              </Button>
              <Button danger icon={<DeleteOutlined />} onClick={clearAllServers}>
                {t('pages.xray.dns.clearAll')}
              </Button>
            </Space>
            <Table
              columns={dnsColumns}
              dataSource={dnsServers}
              rowKey={(r) => r.key}
              pagination={false}
              size="small"
              bordered
            />
          </Space>
        ),
      });

      out.push({
        key: '3',
        label: catTabLabel(<ExperimentOutlined />, 'Fake DNS', isMobile),
        children: fakeDnsList.length === 0 ? (
          <Empty description={t('emptyFakeDnsDesc')}>
            <Button type="primary" icon={<PlusOutlined />} onClick={addFakedns}>
              {t('pages.xray.fakedns.add')}
            </Button>
          </Empty>
        ) : (
          <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
            <Button type="primary" icon={<PlusOutlined />} onClick={addFakedns}>
              {t('pages.xray.fakedns.add')}
            </Button>
            <Table
              columns={fakednsColumns}
              dataSource={fakeDnsList}
              rowKey={(r) => r.key}
              pagination={false}
              size="small"
              bordered
            />
          </Space>
        ),
      });
    }

    return out;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [t, isMobile, dnsEnabled, dns, hostsList, dnsServers, fakeDnsList]);

  return (
    <>
      {modalContextHolder}
      <Tabs defaultActiveKey="1" items={items} />
      <DnsServerModal
        open={serverModalOpen}
        server={editingServer}
        isEdit={editingIndex != null}
        onClose={() => setServerModalOpen(false)}
        onConfirm={onServerConfirm}
      />
      <DnsPresetsModal
        open={presetsModalOpen}
        onClose={() => setPresetsModalOpen(false)}
        onInstall={onPresetInstall}
      />
    </>
  );
}
