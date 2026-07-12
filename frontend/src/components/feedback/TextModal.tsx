import { useEffect, useState } from 'react';
import { Button, Input, Modal, Tabs, message } from 'antd';
import { CopyOutlined, DownloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

import JsonEditor from '@/components/form/JsonEditor';
import { ClipboardManager, FileManager } from '@/utils';

export interface TextModalTab {
  key: string;
  label: string;
  content: string;
}

interface TextModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  content: string;
  fileName?: string;
  json?: boolean;
  tabs?: TextModalTab[];
}

export default function TextModal({ open, onClose, title, content, fileName = '', json = false, tabs }: TextModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [activeKey, setActiveKey] = useState('');

  useEffect(() => {
    if (open && tabs && tabs.length > 0) setActiveKey(tabs[0].key);
  }, [open, tabs]);

  const activeTab = tabs?.find((tab) => tab.key === activeKey) ?? tabs?.[0];
  const activeContent = activeTab ? activeTab.content : content;

  async function copy() {
    const ok = await ClipboardManager.copyText(activeContent || '');
    if (ok) {
      messageApi.success(t('copied'));
      onClose();
    }
  }

  function download() {
    if (!fileName) return;
    FileManager.downloadTextFile(activeContent, fileName);
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={title}
        onCancel={onClose}
        destroyOnHidden
      footer={(
        <>
          {fileName && (
            <Button icon={<DownloadOutlined />} onClick={download}>{fileName}</Button>
          )}
          <Button type="primary" icon={<CopyOutlined />} onClick={copy}>{t('copy')}</Button>
        </>
      )}
    >
      {tabs && tabs.length > 0 && (
        <Tabs
          activeKey={activeTab?.key}
          onChange={setActiveKey}
          items={tabs.map((tab) => ({ key: tab.key, label: tab.label }))}
        />
      )}
      {json ? (
        <JsonEditor value={activeContent} readOnly minHeight="240px" maxHeight="60vh" />
      ) : (
        <Input.TextArea
          value={activeContent}
          readOnly
          autoSize={{ minRows: 10, maxRows: 20 }}
          style={{
            fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
            fontSize: 12,
            overflowY: 'auto',
          }}
        />
      )}
      </Modal>
    </>
  );
}
