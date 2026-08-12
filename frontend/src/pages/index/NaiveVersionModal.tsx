import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Empty, Modal, Radio, Skeleton, Tag } from 'antd';
import { CheckCircleFilled, CloudDownloadOutlined } from '@ant-design/icons';

import { keys } from '@/api/queryKeys';
import { HttpUtil } from '@/utils';
import './NaiveVersionModal.css';

interface BusyEvent {
  busy: boolean;
  tip?: string;
}

interface NaiveRelease {
  tag_name: string;
  published_at?: string;
  prerelease?: boolean;
  installable?: boolean;
}

interface NaiveStatusResponse {
  installed: boolean;
  version?: string;
  releaseTag?: string;
  instances: Array<{ tag: string; running: boolean }>;
}

interface NaiveVersionModalProps {
  open: boolean;
  isMobile: boolean;
  onClose: () => void;
  onBusy: (event: BusyEvent) => void;
}

async function fetchReleases(): Promise<NaiveRelease[]> {
  const msg = await HttpUtil.get<NaiveRelease[]>(
    '/panel/api/naive/releases',
    undefined,
    { silent: true },
  );
  if (!msg.success || !Array.isArray(msg.obj)) {
    throw new Error(msg.msg || 'Naive releases unavailable');
  }
  return msg.obj;
}

async function fetchStatus(): Promise<NaiveStatusResponse> {
  const msg = await HttpUtil.get<NaiveStatusResponse>(
    '/panel/api/naive/status',
    undefined,
    { silent: true },
  );
  if (!msg.success || !msg.obj) {
    throw new Error(msg.msg || 'Naive status unavailable');
  }
  return msg.obj;
}

function normalizeVersion(raw: string): string {
  return raw.trim().toLowerCase().replace(/^naive\s+/, '').replace(/^v/, '').replace(/-\d+$/, '');
}

function compareVersions(left: string, right: string): number {
  const leftParts = normalizeVersion(left).split('.').map((part) => Number(part) || 0);
  const rightParts = normalizeVersion(right).split('.').map((part) => Number(part) || 0);
  const count = Math.max(leftParts.length, rightParts.length);
  for (let index = 0; index < count; index += 1) {
    const difference = (leftParts[index] ?? 0) - (rightParts[index] ?? 0);
    if (difference !== 0) return difference;
  }
  return 0;
}

export default function NaiveVersionModal({
  open,
  isMobile,
  onClose,
  onBusy,
}: NaiveVersionModalProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [selected, setSelected] = useState('');
  const releasesQuery = useQuery({
    queryKey: keys.naive.releases(),
    queryFn: fetchReleases,
    enabled: open,
  });
  const statusQuery = useQuery({
    queryKey: keys.naive.status(),
    queryFn: fetchStatus,
    enabled: open,
  });

  const allReleases = useMemo(() => releasesQuery.data ?? [], [releasesQuery.data]);
  const releases = useMemo(
    () => allReleases.filter((release) => release.installable !== false).slice(0, 5),
    [allReleases],
  );
  const preferredTag = useMemo(
    () => (releases.find((release) => !release.prerelease) ?? releases[0])?.tag_name ?? '',
    [releases],
  );
  const currentVersion = statusQuery.data?.version ?? '';
  const exactReleaseTag = statusQuery.data?.releaseTag ?? '';
  const installedTag = useMemo(() => {
    if (exactReleaseTag && allReleases.some((release) => release.tag_name === exactReleaseTag)) {
      return exactReleaseTag;
    }
    const normalized = normalizeVersion(currentVersion);
    if (!normalized) return '';
    const matches = allReleases.filter(
      (release) => release.installable !== false && normalizeVersion(release.tag_name) === normalized,
    );
    return matches.length === 1 ? matches[0].tag_name : '';
  }, [allReleases, currentVersion, exactReleaseTag]);

  const installed = statusQuery.data?.installed ?? false;

  useEffect(() => {
    if (!open || !preferredTag) return;
    if (!installed) {
      setSelected(preferredTag);
      return;
    }
    if (!installedTag) {
      setSelected('');
      return;
    }
    setSelected(compareVersions(preferredTag, installedTag) > 0 ? preferredTag : installedTag);
  }, [installed, installedTag, open, preferredTag]);

  async function installSelected() {
    if (!selected || selected === installedTag) return;
    onBusy({ busy: true, tip: t('pages.index.dontRefresh') });
    try {
      const msg = await HttpUtil.post(
        '/panel/api/naive/install',
        { version: selected },
        { headers: { 'Content-Type': 'application/json' } },
      );
      if (!msg.success) return;
      await queryClient.invalidateQueries({ queryKey: keys.naive.status() });
      onClose();
    } finally {
      onBusy({ busy: false });
    }
  }

  const error = releasesQuery.error ?? statusQuery.error;
  const loading = releasesQuery.isLoading || statusQuery.isLoading;

  return (
    <Modal
      open={open}
      title={t('pages.xray.naive.sectionTitle')}
      width={isMobile ? '100%' : 620}
      style={isMobile ? { top: 12, maxWidth: 'calc(100vw - 16px)' } : undefined}
      okText={t('pages.xray.naive.install')}
      okButtonProps={{ disabled: loading || !!error || !selected || selected === installedTag }}
      onOk={() => { void installSelected(); }}
      onCancel={onClose}
      destroyOnHidden
    >
      <div className="naive-version-modal">
        <Alert
          type="warning"
          showIcon
          title={t('pages.xray.naive.versionWarning')}
        />

        {error && (
          <Alert
            type="error"
            showIcon
            title={error instanceof Error ? error.message : t('somethingWentWrong')}
          />
        )}

        {loading ? (
          <Skeleton active paragraph={{ rows: 5 }} />
        ) : releases.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('noData')} />
        ) : (
          <Radio.Group
            className="naive-release-list"
            value={selected}
            onChange={(event) => setSelected(event.target.value as string)}
          >
            {releases.map((release, index) => {
              const isInstalled = release.tag_name === installedTag;
              return (
                <label
                  className={
                    `naive-release-option${selected === release.tag_name ? ' is-selected' : ''}`
                  }
                  key={release.tag_name}
                >
                  <Radio value={release.tag_name} />
                  <span className="naive-release-icon"><CloudDownloadOutlined /></span>
                  <span className="naive-release-copy">
                    <span className="naive-release-version">{release.tag_name}</span>
                    <span>
                      {release.published_at
                        ? new Date(release.published_at).toLocaleDateString()
                        : `#${index + 1}`}
                    </span>
                  </span>
                  {release.prerelease && <Tag color="warning">pre</Tag>}
                  {isInstalled && (
                    <CheckCircleFilled
                      className="naive-release-installed"
                      aria-label={t('pages.index.xraySwitch')}
                    />
                  )}
                </label>
              );
            })}
          </Radio.Group>
        )}
      </div>
    </Modal>
  );
}
