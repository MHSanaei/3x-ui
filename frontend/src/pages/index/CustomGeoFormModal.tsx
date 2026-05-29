import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Input, message, Modal, Select } from 'antd';

import { HttpUtil } from '@/utils';
import { CustomGeoFormSchema } from '@/schemas/xray';

export interface CustomGeoRecord {
  id: number;
  type: 'geosite' | 'geoip';
  alias: string;
  url: string;
}

interface CustomGeoFormModalProps {
  open: boolean;
  record: CustomGeoRecord | null;
  onClose: () => void;
  onSaved: () => void;
}

export default function CustomGeoFormModal({
  open,
  record,
  onClose,
  onSaved,
}: CustomGeoFormModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [type, setType] = useState<'geosite' | 'geoip'>('geosite');
  const [alias, setAlias] = useState('');
  const [url, setUrl] = useState('');
  const [saving, setSaving] = useState(false);

  const editing = record != null;

  useEffect(() => {
    if (!open) return;
    if (record) {
      setType(record.type);
      setAlias(record.alias);
      setUrl(record.url);
    } else {
      setType('geosite');
      setAlias('');
      setUrl('');
    }
  }, [open, record]);

  async function submit() {
    const validated = CustomGeoFormSchema.safeParse({ type, alias, url });
    if (!validated.success) {
      messageApi.error(t(validated.error.issues[0]?.message ?? 'somethingWentWrong'));
      return;
    }
    setSaving(true);
    try {
      const apiUrl = editing
        ? `/panel/api/custom-geo/update/${record!.id}`
        : '/panel/api/custom-geo/add';
      const msg = await HttpUtil.post(apiUrl, validated.data);
      if (msg?.success) {
        onSaved();
        onClose();
      }
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={editing ? t('pages.index.customGeoModalEdit') : t('pages.index.customGeoModalAdd')}
      confirmLoading={saving}
      okText={t('pages.index.customGeoModalSave')}
      cancelText={t('close')}
      onOk={submit}
      onCancel={onClose}
    >
      <Form layout="vertical">
        <Form.Item label={t('pages.index.customGeoType')}>
          <Select
            value={type}
            disabled={editing}
            onChange={(v) => setType(v)}
            options={[
              { value: 'geosite', label: 'geosite' },
              { value: 'geoip', label: 'geoip' },
            ]}
          />
        </Form.Item>
        <Form.Item label={t('pages.index.customGeoAlias')}>
          <Input
            value={alias}
            disabled={editing}
            placeholder={t('pages.index.customGeoAliasPlaceholder')}
            onChange={(e) => setAlias(e.target.value)}
          />
        </Form.Item>
        <Form.Item label={t('pages.index.customGeoUrl')}>
          <Input
            value={url}
            placeholder="https://"
            onChange={(e) => setUrl(e.target.value)}
          />
        </Form.Item>
      </Form>
      </Modal>
    </>
  );
}
