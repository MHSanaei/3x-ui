import { useTranslation } from 'react-i18next';
import { Input, InputNumber, Select, Switch } from 'antd';

import type { AllSetting } from '@/models/setting';
import { SettingListItem } from '@/components/ui';

interface ResellerTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

// ResellerTab groups the reseller economy controls — what creating a client
// costs a non-admin user, and the ZarinPal gateway used to top up balances.
export default function ResellerTab({ allSetting, updateSetting }: ResellerTabProps) {
  const { t } = useTranslation();

  return (
    <>
      <SettingListItem
        paddings="small"
        title={t('pages.settings.security.clientCost')}
        description={t('pages.settings.security.clientCostDesc')}
      >
        <InputNumber
          min={0}
          value={allSetting.clientCost}
          onChange={(value) => updateSetting({ clientCost: Number(value) || 0 })}
        />
      </SettingListItem>

      <SettingListItem
        paddings="small"
        title={t('pages.settings.security.clientCostPerGB')}
        description={t('pages.settings.security.clientCostPerGBDesc')}
      >
        <InputNumber
          min={0}
          value={allSetting.clientCostPerGB}
          onChange={(value) => updateSetting({ clientCostPerGB: Number(value) || 0 })}
        />
      </SettingListItem>

      <SettingListItem
        paddings="small"
        title={t('pages.settings.security.zarinpalEnable')}
        description={t('pages.settings.security.zarinpalEnableDesc')}
      >
        <Switch
          checked={allSetting.zarinpalEnable}
          onChange={(checked) => updateSetting({ zarinpalEnable: checked })}
        />
      </SettingListItem>

      <SettingListItem
        paddings="small"
        title={t('pages.settings.security.zarinpalMerchantId')}
        description={t('pages.settings.security.zarinpalMerchantIdDesc')}
      >
        <Input
          style={{ maxWidth: 340 }}
          value={allSetting.zarinpalMerchantId}
          onChange={(e) => updateSetting({ zarinpalMerchantId: e.target.value })}
          placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
        />
      </SettingListItem>

      <SettingListItem
        paddings="small"
        title={t('pages.settings.security.zarinpalCurrency')}
        description={t('pages.settings.security.zarinpalCurrencyDesc')}
      >
        <Select
          style={{ width: 140 }}
          value={allSetting.zarinpalCurrency || 'IRT'}
          onChange={(value) => updateSetting({ zarinpalCurrency: value })}
          options={[
            { value: 'IRT', label: 'IRT (Toman)' },
            { value: 'IRR', label: 'IRR (Rial)' },
          ]}
        />
      </SettingListItem>

      <SettingListItem
        paddings="small"
        title={t('pages.settings.security.zarinpalSandbox')}
        description={t('pages.settings.security.zarinpalSandboxDesc')}
      >
        <Switch
          checked={allSetting.zarinpalSandbox}
          onChange={(checked) => updateSetting({ zarinpalSandbox: checked })}
        />
      </SettingListItem>
    </>
  );
}
