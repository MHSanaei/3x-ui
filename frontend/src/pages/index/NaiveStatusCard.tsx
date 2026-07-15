import { useTranslation } from 'react-i18next';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Badge, Card, Modal, Space } from 'antd';
import {
  PoweroffOutlined,
  ReloadOutlined,
  ToolOutlined,
} from '@ant-design/icons';

import { keys } from '@/api/queryKeys';
import { HttpUtil } from '@/utils';
import { activateOnKey } from '@/utils/a11y';

interface BusyEvent {
  busy: boolean;
  tip?: string;
}

interface NaiveInstance {
  tag: string;
  running: boolean;
  uptimeSeconds: number;
  error?: string;
}

interface NaiveStatusResponse {
  installed: boolean;
  version?: string;
  instances: NaiveInstance[];
}

interface NaiveStatusCardProps {
  isMobile: boolean;
  onOpenVersionModal: () => void;
  onBusy: (event: BusyEvent) => void;
}

async function fetchNaiveStatus(): Promise<NaiveStatusResponse> {
  const msg = await HttpUtil.get<NaiveStatusResponse>('/panel/api/naive/status', undefined, { silent: true });
  if (!msg?.success || !msg.obj) {
    throw new Error(msg?.msg || 'Failed to load naive status');
  }
  return msg.obj;
}

export default function NaiveStatusCard({ isMobile, onOpenVersionModal, onBusy }: NaiveStatusCardProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [modal, modalContextHolder] = Modal.useModal();
  const statusQuery = useQuery({
    queryKey: keys.naive.status(),
    queryFn: fetchNaiveStatus,
    refetchInterval: 5000,
  });

  const data = statusQuery.data ?? { installed: false, version: '', instances: [] };
  const runningCount = data.instances.filter((instance) => instance.running).length;

  async function stopAll() {
    onBusy({ busy: true, tip: t('pages.index.dontRefresh') });
    try {
      const msg = await HttpUtil.post('/panel/api/naive/stop-all', {}, {
        headers: { 'Content-Type': 'application/json' },
      });
      if (msg?.success) {
        await queryClient.invalidateQueries({ queryKey: keys.naive.status() });
      }
    } finally {
      onBusy({ busy: false });
    }
  }

  async function restartAll() {
    onBusy({ busy: true, tip: t('pages.index.dontRefresh') });
    try {
      const msg = await HttpUtil.post('/panel/api/naive/restart-all', {}, {
        headers: { 'Content-Type': 'application/json' },
      });
      if (msg?.success) {
        await queryClient.invalidateQueries({ queryKey: keys.naive.status() });
      }
    } finally {
      onBusy({ busy: false });
    }
  }

  function uninstall() {
    modal.confirm({
      title: t('pages.xray.naive.uninstall'),
      content: t('pages.xray.naive.uninstallConfirm'),
      okText: t('confirm'),
      cancelText: t('cancel'),
      onOk: async () => {
        onBusy({ busy: true, tip: t('pages.index.dontRefresh') });
        try {
          await HttpUtil.post('/panel/api/naive/binary/delete');
          await queryClient.invalidateQueries({ queryKey: keys.naive.status() });
        } finally {
          onBusy({ busy: false });
        }
      },
    });
  }

  const extra = !data.installed
    ? <Badge status="default" text={t('pages.xray.naive.notInstalled')} />
    : runningCount > 0
      ? <Badge status="processing" color="green" text={`${t('pages.index.xrayStatusRunning')} (${runningCount})`} />
      : <Badge status="warning" text={t('pages.index.xrayStatusStop')} />;

  const actions = !data.installed
    ? [
        <Space className="action" key="install" role="button" tabIndex={0} aria-label={t('pages.xray.naive.install')} onClick={onOpenVersionModal} onKeyDown={activateOnKey(onOpenVersionModal)}>
          <ToolOutlined />
          {!isMobile && <span>{t('pages.xray.naive.install')}</span>}
        </Space>,
      ]
    : [
        <Space className="action" key="stop" role="button" tabIndex={0} aria-label={t('pages.xray.naive.stop')} onClick={stopAll} onKeyDown={activateOnKey(stopAll)}>
          <PoweroffOutlined />
          {!isMobile && <span>{t('pages.xray.naive.stop')}</span>}
        </Space>,
        <Space className="action" key="restart" role="button" tabIndex={0} aria-label={t('pages.xray.naive.restartAll')} onClick={restartAll} onKeyDown={activateOnKey(restartAll)}>
          <ReloadOutlined />
          {!isMobile && <span>{t('pages.xray.naive.restartAll')}</span>}
        </Space>,
        <Space className="action" key="update" role="button" tabIndex={0} aria-label={t('pages.xray.naive.install')} onClick={onOpenVersionModal} onKeyDown={activateOnKey(onOpenVersionModal)}>
          <ToolOutlined />
          {!isMobile && <span>{data.version || t('pages.xray.naive.install')}</span>}
        </Space>,
        <Space className="action" key="uninstall" role="button" tabIndex={0} aria-label={t('pages.xray.naive.uninstall')} onClick={uninstall} onKeyDown={activateOnKey(uninstall)}>
          <PoweroffOutlined />
          {!isMobile && <span>{t('pages.xray.naive.uninstall')}</span>}
        </Space>,
      ];

  return (
    <>
      {modalContextHolder}
      <Card hoverable title={t('pages.xray.naive.sectionTitle')} extra={extra} actions={actions} className="xray-status-card" />
    </>
  );
}
