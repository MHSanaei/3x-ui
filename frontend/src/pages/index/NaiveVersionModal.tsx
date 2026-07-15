import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Collapse, Modal, Radio, Spin, Tag } from 'antd';

import { keys } from '@/api/queryKeys';
import { HttpUtil } from '@/utils';
import './VersionModal.css';

interface BusyEvent {
  busy: boolean;
  tip?: string;
}

interface NaiveRelease {
  tag_name: string;
}

interface NaiveStatusResponse {
  installed: boolean;
  version?: string;
  instances: Array<{ tag: string; running: boolean }>;
}

interface NaiveVersionModalProps {
  open: boolean;
  onClose: () => void;
  onBusy: (event: BusyEvent) => void;
}

async function fetchReleases(): Promise<NaiveRelease[]> {
  const msg = await HttpUtil.get<NaiveRelease[]>('/panel/api/naive/releases', undefined, { silent: true });
  if (!msg?.success || !Array.isArray(msg.obj)) {
    throw new Error(msg?.msg || 'Failed to load naive releases');
  }
  return msg.obj;
}

async function fetchStatus(): Promise<NaiveStatusResponse> {
  const msg = await HttpUtil.get<NaiveStatusResponse>('/panel/api/naive/status', undefined, { silent: true });
  if (!msg?.success || !msg.obj) {
    throw new Error(msg?.msg || 'Failed to load naive status');
  }
  return msg.obj;
}

function normalizeVersion(raw: string): string {
  return raw
    .trim()
    .toLowerCase()
    .replace(/^naive\s+/, '')
    .replace(/^v/, '')
    .replace(/-\d+$/, '');
}

export default function NaiveVersionModal({ open, onClose, onBusy }: NaiveVersionModalProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [selected, setSelected] = useState('');
  const [activeKey, setActiveKey] = useState<string | string[]>('1');
  const releasesQuery = useQuery({ queryKey: keys.naive.releases(), queryFn: fetchReleases, enabled: open });
  const statusQuery = useQuery({ queryKey: keys.naive.status(), queryFn: fetchStatus, enabled: open });

  const currentVersion = statusQuery.data?.version || '';

  const installedTag = useMemo(() => {
    const releases = releasesQuery.data || [];
    const normalizedCurrent = normalizeVersion(currentVersion);
    if (!normalizedCurrent) {
      return '';
    }
    const exact = releases.find((release) => normalizeVersion(release.tag_name) === normalizedCurrent);
    return exact?.tag_name || '';
  }, [releasesQuery.data, currentVersion]);

  useEffect(() => {
    if (open) {
      setSelected(installedTag);
      setActiveKey('1');
    }
  }, [open, installedTag]);

  async function installSelected() {
    if (!selected) {
      return;
    }
    onBusy({ busy: true, tip: t('pages.index.dontRefresh') });
    try {
      await HttpUtil.post('/panel/api/naive/install', { version: selected }, { headers: { 'Content-Type': 'application/json' } });
      await queryClient.invalidateQueries({ queryKey: keys.naive.status() });
      setSelected('');
      onClose();
    } finally {
      onBusy({ busy: false });
    }
  }

  return (
    <Modal
      open={open}
      title={t('pages.xray.naive.sectionTitle')}
      okText={t('pages.xray.naive.install')}
      onOk={installSelected}
      onCancel={onClose}
    >
      <Spin spinning={releasesQuery.isLoading || statusQuery.isLoading}>
        <Collapse
          accordion
          activeKey={activeKey}
          onChange={setActiveKey}
          items={[
            {
              key: '1',
              label: t('pages.xray.naive.sectionTitle'),
              children: (
                <>
                  <Alert
                    type="warning"
                    className="mb-12"
                    title={t('pages.xray.naive.versionWarning')}
                    showIcon
                  />
                  <div className="version-list">
                    {(releasesQuery.data || []).slice(0, 5).map((release, index) => (
                      <div key={release.tag_name} className="version-list-item">
                        <Tag color={index % 2 === 0 ? 'purple' : 'green'}>{release.tag_name}</Tag>
                        <Radio checked={selected === release.tag_name} onClick={() => setSelected(release.tag_name)} />
                      </div>
                    ))}
                  </div>
                </>
              ),
            },
          ]}
        />
      </Spin>
    </Modal>
  );
}
