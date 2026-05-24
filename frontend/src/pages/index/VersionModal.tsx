import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Collapse, Modal, Radio, Spin, Tag, Tooltip } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';

import { HttpUtil } from '@/utils';
import type { Status } from '@/models/status';
import CustomGeoSection from './CustomGeoSection';
import './VersionModal.css';

interface BusyEvent {
  busy: boolean;
  tip?: string;
}

interface VersionModalProps {
  open: boolean;
  status: Status;
  onClose: () => void;
  onBusy: (e: BusyEvent) => void;
}

const GEOFILES = [
  'geosite.dat',
  'geoip.dat',
  'geosite_IR.dat',
  'geoip_IR.dat',
  'geosite_RU.dat',
  'geoip_RU.dat',
];

export default function VersionModal({ open, status, onClose, onBusy }: VersionModalProps) {
  const { t } = useTranslation();
  const [modal, modalContextHolder] = Modal.useModal();
  const [activeKey, setActiveKey] = useState<string | string[]>('1');
  const [versions, setVersions] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);

  const fetchVersions = useCallback(async () => {
    setLoading(true);
    try {
      const msg = await HttpUtil.get('/panel/api/server/getXrayVersion');
      if (msg?.success) setVersions(msg.obj || []);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (open) fetchVersions();
  }, [open, fetchVersions]);

  function switchXrayVersion(version: string) {
    modal.confirm({
      title: t('pages.index.xraySwitchVersionDialog'),
      content: t('pages.index.xraySwitchVersionDialogDesc').replace('#version#', version),
      okText: t('confirm'),
      cancelText: t('cancel'),
      onOk: async () => {
        onClose();
        onBusy({ busy: true, tip: t('pages.index.dontRefresh') });
        try {
          await HttpUtil.post(`/panel/api/server/installXray/${version}`);
        } finally {
          onBusy({ busy: false });
        }
      },
    });
  }

  function updateGeofile(fileName: string) {
    const isSingle = !!fileName;
    modal.confirm({
      title: t('pages.index.geofileUpdateDialog'),
      content: isSingle
        ? t('pages.index.geofileUpdateDialogDesc').replace('#filename#', fileName)
        : t('pages.index.geofilesUpdateDialogDesc'),
      okText: t('confirm'),
      cancelText: t('cancel'),
      onOk: async () => {
        onClose();
        onBusy({ busy: true, tip: t('pages.index.dontRefresh') });
        const url = isSingle
          ? `/panel/api/server/updateGeofile/${fileName}`
          : '/panel/api/server/updateGeofile';
        try {
          await HttpUtil.post(url);
        } finally {
          onBusy({ busy: false });
        }
      },
    });
  }

  const activeKeyStr = Array.isArray(activeKey) ? activeKey[0] : activeKey;

  return (
    <Modal
      open={open}
      title={t('pages.index.xrayUpdates')}
      footer={null}
      onCancel={onClose}
    >
      {modalContextHolder}
      <Spin spinning={loading}>
        <Collapse
          accordion
          activeKey={activeKey}
          onChange={setActiveKey}
          items={[
            {
              key: '1',
              label: 'Xray',
              children: (
                <>
                  <Alert
                    type="warning"
                    className="mb-12"
                    title={t('pages.index.xraySwitchClickDesk')}
                    showIcon
                  />
                  <div className="version-list">
                    {versions.map((version, index) => (
                      <div key={version} className="version-list-item">
                        <Tag color={index % 2 === 0 ? 'purple' : 'green'}>{version}</Tag>
                        <Radio
                          checked={version === `v${status?.xray?.version}`}
                          onClick={() => switchXrayVersion(version)}
                        />
                      </div>
                    ))}
                  </div>
                </>
              ),
            },
            {
              key: '2',
              label: 'Geofiles',
              children: (
                <>
                  <div className="version-list">
                    {GEOFILES.map((file, index) => (
                      <div key={file} className="version-list-item">
                        <Tag color={index % 2 === 0 ? 'purple' : 'green'}>{file}</Tag>
                        <Tooltip title={t('update')}>
                          <ReloadOutlined
                            className="reload-icon"
                            onClick={() => updateGeofile(file)}
                          />
                        </Tooltip>
                      </div>
                    ))}
                  </div>
                  <div className="actions-row">
                    <Button onClick={() => updateGeofile('')}>
                      {t('pages.index.geofilesUpdateAll')}
                    </Button>
                  </div>
                </>
              ),
            },
            {
              key: '3',
              label: t('pages.index.customGeoTitle'),
              children: <CustomGeoSection active={activeKeyStr === '3'} />,
            },
          ]}
        />
      </Spin>
    </Modal>
  );
}
