import { Alert, Modal } from 'antd';
import { useTranslation } from 'react-i18next';

interface VersionModalProps {
  open: boolean;
  onClose: () => void;
  onUpload: () => void;
}

export default function VersionModal({ open, onClose, onUpload }: VersionModalProps) {
  const { t } = useTranslation();
  return (
    <Modal
      open={open}
      title={t('pages.index.geodataTitle')}
      okText={t('pages.index.offlineUpdate')}
      onOk={onUpload}
      onCancel={onClose}
    >
      <Alert type="info" showIcon title={t('pages.index.offlineUpdateOnly')} />
    </Modal>
  );
}
