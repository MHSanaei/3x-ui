import { Tag, Tooltip, Typography } from 'antd';
import { useTranslation } from 'react-i18next';

import { REMARK_VARIABLES, REMARK_VAR_GROUPS, wrapToken } from '@/lib/remark/remarkVariables';

interface RemarkVarPickerProps {
  /** Called with the bare token (e.g. "EMAIL") when a chip is clicked. */
  onPick: (token: string) => void;
}

/**
 * RemarkVarPicker is the grouped, tooltipped chip list of {{VAR}} tokens used by
 * the global remark-template field.
 */
export default function RemarkVarPicker({ onPick }: RemarkVarPickerProps) {
  const { t } = useTranslation();
  return (
    <div style={{ maxWidth: 460, maxHeight: 'min(70vh, 640px)', overflowY: 'auto' }}>
      <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 8 }}>
        {t('pages.hosts.remarkVars.intro')}
      </Typography.Paragraph>
      {REMARK_VAR_GROUPS.map((group) => (
        <div key={group} style={{ marginBottom: 8 }}>
          <div style={{ fontSize: 11, fontWeight: 600, textTransform: 'uppercase', opacity: 0.6, marginBottom: 4 }}>
            {t(`pages.hosts.remarkVars.groups.${group}`)}
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
            {REMARK_VARIABLES.filter((v) => v.group === group).map((v) => (
              <Tooltip key={v.token} title={t(`pages.hosts.remarkVars.desc${v.token}`)}>
                <Tag
                  onClick={() => onPick(v.token)}
                  style={{ cursor: 'pointer', margin: 0, fontFamily: 'monospace' }}
                >
                  {wrapToken(v.token)}
                </Tag>
              </Tooltip>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
