import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Input, Modal, QRCode, message } from 'antd';
import * as OTPAuth from 'otpauth';

import { ClipboardManager } from '@/utils';
import { activateOnKey } from '@/utils/a11y';
import { TotpCodeSchema } from '@/schemas/login';
import './TwoFactorModal.css';

type Type = 'set' | 'confirm';

interface TwoFactorModalProps {
  open: boolean;
  title?: string;
  description?: string;
  token?: string;
  type?: Type;
  onConfirm: (success: boolean, code?: string) => void;
  onOpenChange: (open: boolean) => void;
}

export default function TwoFactorModal({
  open,
  title = '',
  description = '',
  token = '',
  type = 'set',
  onConfirm,
  onOpenChange,
}: TwoFactorModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [enteredCode, setEnteredCode] = useState('');
  const [qrValue, setQrValue] = useState('');
  const totpRef = useRef<OTPAuth.TOTP | null>(null);

  useEffect(() => {
    if (!open) return;
     
    setEnteredCode('');
    totpRef.current = null;
    setQrValue('');
    if (token) {
      const totp = new OTPAuth.TOTP({
        issuer: '3x-ui',
        label: 'Administrator',
        algorithm: 'SHA1',
        digits: 6,
        period: 30,
        secret: token,
      });
      totpRef.current = totp;
      setQrValue(totp.toString());
    }
     
  }, [open, token]);

  function close(success: boolean, code = '') {
    onConfirm(success, code);
    onOpenChange(false);
    setEnteredCode('');
  }

  function onOk() {
    const codeOk = TotpCodeSchema.safeParse(enteredCode);
    if (!codeOk.success) {
      messageApi.error(t(codeOk.error.issues[0]?.message ?? 'pages.settings.security.twoFactorModalError'));
      return;
    }
    if (type === 'confirm' && !token) {
      close(true, codeOk.data);
      return;
    }
    if (!totpRef.current) return;
    if (totpRef.current.generate() === codeOk.data) {
      close(true);
    } else {
      messageApi.error(t('pages.settings.security.twoFactorModalError'));
    }
  }

  function onCancel() {
    close(false);
  }

  async function copyToken() {
    const ok = await ClipboardManager.copyText(token);
    if (ok) messageApi.success(t('copied'));
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={title}
        closable
        onCancel={onCancel}
      footer={[
        <Button key="cancel" onClick={onCancel}>{t('cancel')}</Button>,
        <Button key="ok" type="primary" disabled={!TotpCodeSchema.safeParse(enteredCode).success} onClick={onOk}>
          {t('confirm')}
        </Button>,
      ]}
    >
      {type === 'set' ? (
        <>
          <p>{t('pages.settings.security.twoFactorModalSteps')}</p>
          <Divider />
          <p>{t('pages.settings.security.twoFactorModalFirstStep')}</p>
          <div
            className="qr-wrap"
            role="button"
            tabIndex={0}
            aria-label={t('copy')}
            onClick={copyToken}
            onKeyDown={activateOnKey(copyToken)}
          >
            <QRCode
              className="qr-code"
              value={qrValue}
              size={180}
              type="svg"
              bordered={false}
              color="#000000"
              bgColor="#ffffff"
              errorLevel="L"
              title={t('copy')}
            />
            <span className="qr-token">{token}</span>
          </div>
          <Divider />
          <p>{t('pages.settings.security.twoFactorModalSecondStep')}</p>
          <Input value={enteredCode} onChange={(e) => setEnteredCode(e.target.value)} style={{ width: '100%' }} aria-label={t('twoFactorCode')} />
        </>
      ) : (
        <>
          <p>{description}</p>
          <Input value={enteredCode} onChange={(e) => setEnteredCode(e.target.value)} style={{ width: '100%' }} aria-label={t('twoFactorCode')} />
        </>
      )}
      </Modal>
    </>
  );
}
