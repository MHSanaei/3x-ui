import { Alert, Button, Flex, Select, Spin, Typography } from 'antd';
import { useTranslation } from 'react-i18next';

import { useNetworkInterfaces } from '@/api/queries/useNetworkInterfaces';
import { SettingListItem } from '@/components/ui';
import type { AllSetting } from '@/models/setting';

export default function NetworkTrafficSettings({
  allSetting,
  updateSetting,
}: {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}) {
  const { t } = useTranslation();
  const mode = allSetting.networkTrafficMode;
  const query = useNetworkInterfaces(mode !== 'legacy');
  const interfaces = query.data?.interfaces ?? [];
  const missing =
    mode === 'manual' &&
    query.data &&
    !interfaces.some((iface) => iface.name === allSetting.networkTrafficInterface);
  return (
    <SettingListItem
      paddings="small"
      title={t('pages.settings.networkTrafficInterfaceTitle')}
      description={t('pages.settings.networkTrafficDescription')}
    >
      <Flex vertical gap="small" style={{ width: '100%' }}>
        <Select
          id="network-traffic-mode"
          aria-label={t('pages.settings.networkTrafficMode')}
          value={mode}
          options={[
            { value: 'legacy', label: t('pages.settings.networkTrafficLegacy') },
            { value: 'auto', label: t('pages.settings.networkTrafficAutomatic') },
            { value: 'manual', label: t('pages.settings.networkTrafficManual') },
          ]}
          onChange={(value) =>
            updateSetting({ networkTrafficMode: value, networkTrafficInterface: '' })
          }
        />
        {mode === 'manual' && (
          <Select
            id="network-traffic-interface"
            aria-label={t('pages.settings.networkTrafficInterfaceTitle')}
            showSearch
            value={allSetting.networkTrafficInterface || undefined}
            placeholder={t('pages.settings.networkTrafficInterfaceTitle')}
            loading={query.isFetching}
            disabled={query.isPending || query.isError}
            options={interfaces.map((iface) => ({
              value: iface.name,
              title: iface.name,
              label: iface.up ? iface.name : `${iface.name} (${t('disabled')})`,
            }))}
            notFoundContent={
              query.isPending ? <Spin size="small" /> : t('pages.settings.networkTrafficEmpty')
            }
            onChange={(value) => updateSetting({ networkTrafficInterface: value })}
          />
        )}
        {mode !== 'legacy' && query.isPending && <Spin size="small" aria-label={t('loading')} />}
        {mode !== 'legacy' && query.isError && (
          <Alert
            type="error"
            showIcon
            title={t('pages.settings.networkTrafficLoadError')}
            action={
              <Button size="small" onClick={() => void query.refetch()}>
                {t('refresh')}
              </Button>
            }
          />
        )}
        {missing && (
          <Alert type="warning" showIcon title={t('pages.settings.networkTrafficUnavailable')} />
        )}
        {mode === 'manual' && query.data && interfaces.length === 0 && (
          <Typography.Text type="secondary">
            {t('pages.settings.networkTrafficEmpty')}
          </Typography.Text>
        )}
        {mode === 'auto' &&
          query.data &&
          (query.data.automatic ? (
            <Typography.Text type="secondary">
              {t('pages.settings.networkTrafficAutoResolved', { interface: query.data.automatic })}
            </Typography.Text>
          ) : (
            <Alert
              type="warning"
              showIcon
              title={t('pages.settings.networkTrafficAutoUnavailable')}
              description={query.data.automaticError}
            />
          ))}
      </Flex>
    </SettingListItem>
  );
}
