import { Button, Input, Modal, message } from 'antd';
import { CopyOutlined, DownloadOutlined } from '@ant-design/icons';

import { ClipboardManager, FileManager } from '@/utils';

interface TextModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  content: string;
  fileName?: string;
}

export default function TextModal({ open, onClose, title, content, fileName = '' }: TextModalProps) {
  const [messageApi, messageContextHolder] = message.useMessage();
  async function copy() {
    const ok = await ClipboardManager.copyText(content || '');
    if (ok) {
      messageApi.success('Copied');
      onClose();
    }
  }

  function download() {
    if (!fileName) return;
    FileManager.downloadTextFile(content, fileName);
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
          <Button type="primary" icon={<CopyOutlined />} onClick={copy}>Copy</Button>
        </>
      )}
    >
      <Input.TextArea
        value={content}
        readOnly
        autoSize={{ minRows: 10, maxRows: 20 }}
        style={{
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
          fontSize: 12,
          overflowY: 'auto',
        }}
      />
      </Modal>
    </>
  );
}
