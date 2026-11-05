import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Modal, Typography } from 'antd';

import type { GroupSummary } from '@/schemas/client';
import HostGroupSelect from './HostGroupSelect';

interface GroupHostRulesModalProps {
  open: boolean;
  group: GroupSummary | null;
  onOpenChange: (open: boolean) => void;
  save: (name: string, hostGroupIds: string[]) => Promise<boolean>;
}

export default function GroupHostRulesModal({
  open,
  group,
  onOpenChange,
  save,
}: GroupHostRulesModalProps) {
  const { t } = useTranslation();
  const [loadingSave, setLoadingSave] = useState(false);
  const openGroupKey = open ? (group?.name ?? null) : null;
  const [syncedGroupKey, setSyncedGroupKey] = useState(openGroupKey);
  const [value, setValue] = useState<string[]>(group?.hostGroupIds ?? []);

  // Reset during render so the first open frame already shows the saved set.
  if (openGroupKey !== syncedGroupKey) {
    setSyncedGroupKey(openGroupKey);
    if (open) {
      setValue(group?.hostGroupIds ?? []);
      setLoadingSave(false);
    }
  }

  async function onOk() {
    if (!group || loadingSave) return;
    setLoadingSave(true);
    try {
      if (await save(group.name, value)) onOpenChange(false);
    } finally {
      setLoadingSave(false);
    }
  }

  return (
    <Modal
      open={open}
      title={t('pages.groups.hostRules')}
      okText={t('save')}
      cancelText={t('cancel')}
      confirmLoading={loadingSave}
      onOk={onOk}
      onCancel={() => onOpenChange(false)}
      destroyOnHidden
    >
      <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
        {t('pages.groups.hostRulesHint')}
      </Typography.Paragraph>
      <HostGroupSelect value={value} onChange={setValue} />
    </Modal>
  );
}
