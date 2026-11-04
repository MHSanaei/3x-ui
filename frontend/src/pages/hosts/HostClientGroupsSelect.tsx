import { useTranslation } from 'react-i18next';
import { Select } from 'antd';

import { selectTagRender } from '@/components/form';
import { FormField } from '@/components/form/rhf';
import { useClientGroupsQuery } from '@/api/queries/useClientGroupsQuery';

export default function HostClientGroupsSelect() {
  const { t } = useTranslation();
  const { groups } = useClientGroupsQuery();
  const options = groups.map((group) => ({ value: group.id, label: group.name }));

  return (
    <FormField
      name="clientGroupIds"
      label={t('pages.hosts.fields.clientGroups')}
      tooltip={t('pages.hosts.hints.clientGroups')}
    >
      <Select
        mode="multiple"
        allowClear
        tagRender={selectTagRender}
        options={options}
        showSearch={{ optionFilterProp: 'label' }}
        placeholder={t('pages.hosts.fields.clientGroups')}
      />
    </FormField>
  );
}
