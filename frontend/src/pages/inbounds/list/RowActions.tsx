import { useTranslation } from 'react-i18next';
import { Button, Dropdown, type MenuProps } from 'antd';
import {
  MoreOutlined,
  EditOutlined,
  CopyOutlined,
  ExportOutlined,
  RetweetOutlined,
  BlockOutlined,
  DeleteOutlined,
  InfoCircleOutlined,
  TagsOutlined,
  UsergroupAddOutlined,
  UsergroupDeleteOutlined,
} from '@ant-design/icons';

import { isInboundMultiUser } from './helpers';
import type { DBInboundRecord, RowAction } from './types';

interface RowActionsMenuProps {
  record: DBInboundRecord;
  subEnable: boolean;
  hasClients: boolean;
  onClick: (key: RowAction) => void;
  isMobile?: boolean;
}

export function buildRowActionsMenu({
  record,
  subEnable,
  t,
  isMobile,
  hasClients,
}: {
  record: DBInboundRecord;
  subEnable: boolean;
  t: (k: string) => string;
  isMobile?: boolean;
  hasClients?: boolean;
}): MenuProps['items'] {
  const items: MenuProps['items'] = [];
  if (isMobile) {
    items.push({ key: 'edit', icon: <EditOutlined aria-hidden />, label: t('edit') });
  }
  if (isInboundMultiUser(record)) {
    items.push({
      key: 'export',
      icon: <ExportOutlined aria-hidden />,
      label: t('pages.inbounds.export'),
    });
    if (subEnable) {
      items.push({
        key: 'subs',
        icon: <ExportOutlined aria-hidden />,
        label: `${t('pages.inbounds.export')} — ${t('pages.settings.subSettings')}`,
      });
    }
  } else {
    items.push({
      key: 'showInfo',
      icon: <InfoCircleOutlined aria-hidden />,
      label: t('pages.inbounds.inboundInfo'),
    });
  }
  items.push({
    key: 'clipboard',
    icon: <CopyOutlined aria-hidden />,
    label: t('pages.inbounds.exportInbound'),
  });
  items.push({
    key: 'resetTraffic',
    icon: <RetweetOutlined aria-hidden />,
    label: t('pages.inbounds.resetTraffic'),
  });
  items.push({
    key: 'clone',
    icon: <BlockOutlined aria-hidden />,
    label: t('pages.inbounds.clone'),
  });
  if (isInboundMultiUser(record)) {
    items.push({
      key: 'attachExisting',
      icon: <UsergroupAddOutlined aria-hidden />,
      label: t('pages.inbounds.attachExistingClients'),
    });
  }
  if (isInboundMultiUser(record) && hasClients) {
    items.push({
      key: 'attachClients',
      icon: <UsergroupAddOutlined aria-hidden />,
      label: t('pages.inbounds.attachClients'),
    });
    items.push({
      key: 'detachClients',
      icon: <UsergroupDeleteOutlined aria-hidden />,
      label: t('pages.inbounds.detachClients'),
    });
    items.push({
      key: 'addToGroup',
      icon: <TagsOutlined aria-hidden />,
      label: t('pages.inbounds.addClientsToGroup'),
    });
    items.push({ type: 'divider' });
    items.push({
      key: 'delAllClients',
      icon: <UsergroupDeleteOutlined aria-hidden />,
      danger: true,
      label: t('pages.inbounds.delAllClients'),
    });
  } else {
    items.push({ type: 'divider' });
  }
  items.push({
    key: 'delete',
    icon: <DeleteOutlined aria-hidden />,
    danger: true,
    label: t('delete'),
  });
  return items;
}

export function RowActionsCell({ record, subEnable, hasClients, onClick }: RowActionsMenuProps) {
  const { t } = useTranslation();
  return (
    <div className="action-buttons">
      <Button
        type="text"
        size="small"
        style={{ fontSize: 16 }}
        icon={<EditOutlined aria-hidden />}
        aria-label={t('edit')}
        onClick={() => onClick('edit')}
      />
      <Dropdown
        trigger={['click']}
        menu={{
          items: buildRowActionsMenu({ record, subEnable, t, hasClients }),
          onClick: ({ key }) => onClick(key as RowAction),
        }}
      >
        <Button
          type="text"
          size="small"
          style={{ fontSize: 16 }}
          icon={<MoreOutlined aria-hidden />}
          aria-label={t('more')}
        />
      </Dropdown>
    </div>
  );
}
