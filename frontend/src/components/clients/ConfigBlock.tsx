import type { MouseEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Collapse, Popover, Tag, Tooltip, message } from 'antd';
import { CopyOutlined, DownloadOutlined, QrcodeOutlined } from '@ant-design/icons';

import { ClipboardManager, FileManager } from '@/utils';
import { QrPanel } from '@/pages/inbounds/qr';
import './ConfigBlock.css';

interface ConfigBlockProps {
  label: string;
  text: string;
  fileName: string;
  qrRemark?: string;
  showQr?: boolean;
  tagColor?: string;
  defaultOpen?: boolean;
}

export default function ConfigBlock({
  label,
  text,
  fileName,
  qrRemark = '',
  showQr = true,
  tagColor = 'gold',
  defaultOpen = false,
}: ConfigBlockProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();

  async function copy() {
    const ok = await ClipboardManager.copyText(text);
    if (ok) messageApi.success(t('copied'));
  }

  const actions = (
    /* eslint-disable-next-line jsx-a11y/no-static-element-interactions, jsx-a11y/click-events-have-key-events */
    <div className="config-block-actions" onClick={(e: MouseEvent) => e.stopPropagation()}>
      <Tooltip title={t('copy')}>
        <Button size="small" icon={<CopyOutlined />} aria-label={t('copy')} onClick={copy} />
      </Tooltip>
      <Tooltip title={t('download')}>
        <Button
          size="small"
          icon={<DownloadOutlined />}
          aria-label={t('download')}
          onClick={() => FileManager.downloadTextFile(text, fileName)}
        />
      </Tooltip>
      {showQr && (
        <Popover
          trigger="click"
          placement="left"
          destroyOnHidden
          content={<QrPanel value={text} remark={qrRemark || label} size={220} />}
        >
          <Tooltip title={t('pages.clients.qrCode')}>
            <Button size="small" icon={<QrcodeOutlined />} aria-label={t('pages.clients.qrCode')} />
          </Tooltip>
        </Popover>
      )}
    </div>
  );

  return (
    <>
      {messageContextHolder}
      <Collapse
        className="config-block"
        collapsible="header"
        defaultActiveKey={defaultOpen ? ['cfg'] : []}
        items={[{
          key: 'cfg',
          label: <Tag color={tagColor} style={{ margin: 0, fontWeight: 600, letterSpacing: '0.3px' }}>{label}</Tag>,
          extra: actions,
          children: <code className="config-block-text">{text}</code>,
        }]}
      />
    </>
  );
}
