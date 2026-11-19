import { InputNumber, Space, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import type { AllSetting } from '@/models/setting';

interface Props {
  rule: 'rate' | 'window';
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

export function UserTrafficAlertInputs({ rule, allSetting, updateSetting }: Props) {
  const { t } = useTranslation();
  const fields =
    rule === 'rate'
      ? [
          { key: 'smtpUserRateMB' as const, label: 'userTrafficRate', max: 100000 },
          { key: 'smtpUserRateSeconds' as const, label: 'userTrafficDuration', max: 86400 },
        ]
      : [
          { key: 'smtpUserWindowGB' as const, label: 'userTrafficWindowLimit', max: 1000000 },
          { key: 'smtpUserWindowMinutes' as const, label: 'userTrafficWindow', max: 1440 },
        ];
  return (
    <Space wrap>
      {fields.map(({ key, label, max }) => (
        <Space key={key} orientation="vertical" size={4}>
          <Typography.Text>{t(`pages.settings.${label}`)}</Typography.Text>
          <InputNumber
            min={1}
            max={max}
            precision={0}
            value={allSetting[key]}
            aria-label={t(`pages.settings.${label}`)}
            onChange={(value) => {
              if (value != null) updateSetting({ [key]: value });
            }}
          />
        </Space>
      ))}
    </Space>
  );
}
