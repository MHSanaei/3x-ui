import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Modal, Table, Tooltip, Typography, message } from 'antd';
import type { TableColumnType } from 'antd';
import { CopyOutlined, DownloadOutlined } from '@ant-design/icons';

import type { ClientRecord } from '@/hooks/useClients';

interface SubSettings {
  enable: boolean;
  subURI: string;
  subJsonURI: string;
  subJsonEnable: boolean;
}

interface SubLinksModalProps {
  open: boolean;
  emails: string[];
  clients: ClientRecord[];
  subSettings?: SubSettings;
  onOpenChange: (open: boolean) => void;
}

interface Row {
  key: string;
  email: string;
  subId: string;
  link: string;
  jsonLink: string;
}

export default function SubLinksModal({
  open,
  emails,
  clients,
  subSettings,
  onOpenChange,
}: SubLinksModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();

  const enabled = !!subSettings?.enable && !!subSettings?.subURI;
  const jsonEnabled = !!subSettings?.subJsonEnable && !!subSettings?.subJsonURI;

  const rows = useMemo<Row[]>(() => {
    if (!enabled) return [];
    const byEmail = new Map(clients.map((c) => [c.email, c]));
    const out: Row[] = [];
    for (const email of emails) {
      const c = byEmail.get(email);
      if (!c?.subId) continue;
      out.push({
        key: email,
        email,
        subId: c.subId,
        link: subSettings!.subURI + c.subId,
        jsonLink: jsonEnabled ? subSettings!.subJsonURI + c.subId : '',
      });
    }
    return out;
  }, [emails, clients, enabled, jsonEnabled, subSettings]);

  const allText = useMemo(
    () => rows.map((r) => (jsonEnabled ? `${r.email}\t${r.link}\t${r.jsonLink}` : `${r.email}\t${r.link}`)).join('\n'),
    [rows, jsonEnabled],
  );

  async function copy(text: string, label?: string) {
    try {
      await navigator.clipboard.writeText(text);
      messageApi.success(label || t('copied'));
    } catch {
      messageApi.error(t('somethingWentWrong'));
    }
  }

  function download() {
    const blob = new Blob([allText], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    const stamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
    a.href = url;
    a.download = `sub-links-${stamp}.txt`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  }

  const columns: TableColumnType<Row>[] = [
    {
      title: t('pages.clients.client'),
      dataIndex: 'email',
      key: 'email',
      width: 180,
      ellipsis: true,
    },
    {
      title: t('pages.clients.subLinkColumn'),
      dataIndex: 'link',
      key: 'link',
      ellipsis: true,
      render: (link: string) => (
        <Tooltip title={link} placement="topLeft">
          <Typography.Text copyable={false} ellipsis>{link}</Typography.Text>
        </Tooltip>
      ),
    },
    {
      title: '',
      key: 'actions',
      width: 64,
      render: (_v, row) => (
        <Button size="small" type="text" icon={<CopyOutlined />} onClick={() => copy(row.link, t('copied'))} />
      ),
    },
  ];

  if (jsonEnabled) {
    columns.splice(2, 0, {
      title: t('pages.clients.subJsonLinkColumn'),
      dataIndex: 'jsonLink',
      key: 'jsonLink',
      ellipsis: true,
      render: (link: string) => (
        <Tooltip title={link} placement="topLeft">
          <Typography.Text copyable={false} ellipsis>{link}</Typography.Text>
        </Tooltip>
      ),
    });
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={t('pages.clients.subLinksTitle', { count: rows.length })}
        width={780}
        footer={
          <div style={{ display: 'flex', justifyContent: 'space-between' }}>
            <Button onClick={() => onOpenChange(false)}>{t('close')}</Button>
            <div style={{ display: 'flex', gap: 8 }}>
              <Button
                icon={<CopyOutlined />}
                disabled={rows.length === 0}
                onClick={() => copy(allText, t('pages.clients.subLinksCopiedAll', { count: rows.length }))}
              >
                {t('pages.clients.subLinksCopyAll')}
              </Button>
              <Button
                type="primary"
                icon={<DownloadOutlined />}
                disabled={rows.length === 0}
                onClick={download}
              >
                {t('download')}
              </Button>
            </div>
          </div>
        }
        onCancel={() => onOpenChange(false)}
      >
        {!enabled && (
          <Alert
            type="warning"
            showIcon
            message={t('pages.clients.subLinksDisabled')}
            description={t('pages.clients.subLinksDisabledHint')}
            style={{ marginBottom: 12 }}
          />
        )}
        {enabled && rows.length === 0 && (
          <Alert
            type="info"
            showIcon
            message={t('pages.clients.subLinksEmpty')}
            style={{ marginBottom: 12 }}
          />
        )}
        {rows.length > 0 && (
          <Table<Row>
            dataSource={rows}
            columns={columns}
            size="small"
            pagination={false}
            scroll={{ y: 360 }}
          />
        )}
      </Modal>
    </>
  );
}
