import { useTranslation } from 'react-i18next';
import { Button, Popover } from 'antd';
import { InfoCircleOutlined } from '@ant-design/icons';

interface PostQuantumQrNoticeProps {
  size?: 'small' | 'middle';
}

export default function PostQuantumQrNotice({ size = 'small' }: PostQuantumQrNoticeProps) {
  const { t } = useTranslation();
  return (
    <Popover
      trigger={['hover', 'focus', 'click']}
      title={t('qrCodeUnavailable')}
      content={<div style={{ maxWidth: 300 }}>{t('qrCodePostQuantumHint')}</div>}
    >
      <Button size={size} icon={<InfoCircleOutlined />} aria-label={t('qrCodeUnavailable')} />
    </Popover>
  );
}
