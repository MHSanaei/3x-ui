import { Select } from 'antd';
import { useTranslation } from 'react-i18next';

import { useHostsQuery } from '@/api/queries/useHostsQuery';
import { selectTagRender } from '@/components/form';

interface HostGroupSelectProps {
  value: string[];
  onChange: (value: string[]) => void;
  ariaLabel?: string;
}

// Shared host-rule picker for the group create, edit, and quick-assign dialogs.
export default function HostGroupSelect({ value, onChange, ariaLabel }: HostGroupSelectProps) {
  const { t } = useTranslation();
  const { hosts, loading } = useHostsQuery();
  // /hosts/list returns one grouped entry per groupId, so option values are unique.
  const options = hosts.map((host) => ({
    value: host.groupId,
    label: host.remark || host.groupId,
  }));

  return (
    <Select
      mode="multiple"
      allowClear
      loading={loading}
      value={value}
      onChange={onChange}
      options={options}
      optionFilterProp="label"
      aria-label={ariaLabel ?? t('pages.groups.hostRules')}
      tagRender={selectTagRender}
      placeholder={t('pages.groups.hostRulesPlaceholder')}
      style={{ width: '100%' }}
    />
  );
}
