import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Input, Modal, QRCode } from 'antd';
import { CopyOutlined, DownloadOutlined, QrcodeOutlined } from '@ant-design/icons';

interface SubQrButtonProps {
  value: string;
  label: string;
  onCopy: (value: string) => void;
}

export default function SubQrButton({ value, label, onCopy }: SubQrButtonProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const qrRef = useRef<HTMLDivElement>(null);

  const saveQr = () => {
    const canvas = qrRef.current?.querySelector('canvas');
    if (!canvas) return;
    const link = document.createElement('a');
    link.href = canvas.toDataURL('image/png');
    link.download = `${label || 'qrcode'}.png`;
    link.click();
  };

  return (
    <>
      <Button icon={<QrcodeOutlined />} aria-label="QR" title="QR" onClick={() => setOpen(true)} />
      <Modal
        open={open}
        onCancel={() => setOpen(false)}
        footer={null}
        width={440}
        centered
        destroyOnHidden
        rootClassName="sub-qr-modal"
        title={t('subscription.qrTitle')}
      >
        <p className="sub-muted sub-qr-modal-hint">{t('subscription.qrHint')}</p>
        <div ref={qrRef} className="sub-qr-modal-code">
          <QRCode
            value={value}
            size={240}
            type="canvas"
            marginSize={2}
            bordered={false}
            color="#000000"
            bgColor="#ffffff"
          />
        </div>
        <Input.TextArea
          className="sub-qr-modal-link"
          value={value}
          readOnly
          dir="ltr"
          autoSize={{ minRows: 2, maxRows: 5 }}
        />
        <div className="sub-qr-modal-actions">
          <Button type="primary" size="large" icon={<CopyOutlined />} onClick={() => onCopy(value)}>
            {t('copy')}
          </Button>
          <Button size="large" icon={<DownloadOutlined />} onClick={saveQr}>
            {t('subscription.saveQr')}
          </Button>
        </div>
      </Modal>
    </>
  );
}
