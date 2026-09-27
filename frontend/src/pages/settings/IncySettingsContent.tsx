import { useTranslation } from 'react-i18next';
import { Input, Select, Switch, Tabs } from 'antd';
import {
  AppstoreOutlined,
  BranchesOutlined,
  NotificationOutlined,
  SafetyOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import type { AllSetting } from '@/models/setting';
import { SettingListItem } from '@/components/ui';
import { catTabLabel } from './catTabLabel';

interface IncySettingsContentProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
  isMobile: boolean;
  remoteSourceBadge: (val: string) => React.ReactNode;
}

// Incy documents every switch as `1`/`0`; an empty value omits the header so
// the subscriber's own app choice wins.
const onOff = (t: (key: string) => string) => [
  { value: '', label: t('pages.settings.subIncyNotSet') },
  { value: '1', label: t('pages.settings.subIncyOn') },
  { value: '0', label: t('pages.settings.subIncyOff') },
];

export default function IncySettingsContent({
  allSetting,
  updateSetting,
  isMobile,
  remoteSourceBadge,
}: IncySettingsContentProps) {
  const { t } = useTranslation();

  return (
    <>
      <SettingListItem
        paddings="small"
        title={t('pages.settings.subIncyAppAutoDetect')}
        description={t('pages.settings.subIncyAppAutoDetectDesc')}
      >
        <Switch
          checked={allSetting.subIncyAppAutoDetect}
          onChange={(v) => updateSetting({ subIncyAppAutoDetect: v })}
        />
      </SettingListItem>

      <Tabs
        type="card"
        size="small"
        items={[
          {
            key: 'app',
            label: catTabLabel(<AppstoreOutlined />, t('pages.settings.subIncyGroupApp'), isMobile),
            children: (
              <>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyProfileDescription')}
                  description={t('pages.settings.subIncyProfileDescriptionDesc')}
                >
                  <Input
                    value={allSetting.subIncyProfileDescription}
                    maxLength={200}
                    onChange={(e) => updateSetting({ subIncyProfileDescription: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncySortOrder')}
                  description={t('pages.settings.subIncySortOrderDesc')}
                >
                  <Select
                    value={allSetting.subIncySortOrder}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subIncySortOrder: v })}
                    options={[
                      { value: '', label: t('pages.settings.subIncyNotSet') },
                      { value: 'none', label: t('pages.settings.subIncySortNone') },
                      { value: 'ping', label: t('pages.settings.subIncySortPing') },
                      { value: 'name', label: t('pages.settings.subIncySortName') },
                    ]}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncySupportEmail')}
                  description={t('pages.settings.subIncySupportEmailDesc')}
                >
                  <Input
                    value={allSetting.subIncySupportEmail}
                    placeholder="support@example.com"
                    onChange={(e) => updateSetting({ subIncySupportEmail: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyAnnounceUrl')}
                  description={t('pages.settings.subIncyAnnounceUrlDesc')}
                >
                  <Input
                    value={allSetting.subIncyAnnounceUrl}
                    placeholder="https://t.me/your_channel"
                    onChange={(e) => updateSetting({ subIncyAnnounceUrl: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyPremiumUrl')}
                  description={t('pages.settings.subIncyPremiumUrlDesc')}
                >
                  <Input
                    value={allSetting.subIncyPremiumUrl}
                    placeholder="https://example.com/buy"
                    onChange={(e) => updateSetting({ subIncyPremiumUrl: e.target.value })}
                  />
                </SettingListItem>
              </>
            ),
          },
          {
            key: 'banners',
            label: catTabLabel(
              <NotificationOutlined />,
              t('pages.settings.subIncyGroupBanners'),
              isMobile,
            ),
            children: (
              <>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyBannerText')}
                  description={t('pages.settings.subIncyBannerTextDesc')}
                >
                  <Input
                    value={allSetting.subIncyBannerText}
                    onChange={(e) => updateSetting({ subIncyBannerText: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyBannerButtonText')}
                  description={t('pages.settings.subIncyBannerButtonTextDesc')}
                >
                  <Input
                    value={allSetting.subIncyBannerButtonText}
                    onChange={(e) => updateSetting({ subIncyBannerButtonText: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyBannerButtonUrl')}
                  description={t('pages.settings.subIncyBannerButtonUrlDesc')}
                >
                  <Input
                    value={allSetting.subIncyBannerButtonUrl}
                    placeholder="https://example.com/sale"
                    onChange={(e) => updateSetting({ subIncyBannerButtonUrl: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyBannerBgColor')}
                  description={t('pages.settings.subIncyBannerBgColorDesc')}
                >
                  <Input
                    value={allSetting.subIncyBannerBgColor}
                    placeholder="#E53E3E"
                    onChange={(e) => updateSetting({ subIncyBannerBgColor: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyBannerButtonColor')}
                  description={t('pages.settings.subIncyBannerButtonColorDesc')}
                >
                  <Input
                    value={allSetting.subIncyBannerButtonColor}
                    placeholder="#38A169"
                    onChange={(e) => updateSetting({ subIncyBannerButtonColor: e.target.value })}
                  />
                </SettingListItem>
              </>
            ),
          },
          {
            key: 'privacy',
            label: catTabLabel(
              <SafetyOutlined />,
              t('pages.settings.subIncyGroupPrivacy'),
              isMobile,
            ),
            children: (
              <>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyHideUrl')}
                  description={t('pages.settings.subIncyHideUrlDesc')}
                >
                  <Select
                    value={allSetting.subIncyHideUrl}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subIncyHideUrl: v })}
                    options={onOff(t)}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyHideCheck')}
                  description={t('pages.settings.subIncyHideCheckDesc')}
                >
                  <Select
                    value={allSetting.subIncyHideCheck}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subIncyHideCheck: v })}
                    options={onOff(t)}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyNoLimit')}
                  description={t('pages.settings.subIncyNoLimitDesc')}
                >
                  <Select
                    value={allSetting.subIncyNoLimitEnabled}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subIncyNoLimitEnabled: v })}
                    options={onOff(t)}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyPerAppEnable')}
                  description={t('pages.settings.subIncyPerAppEnableDesc')}
                >
                  <Select
                    value={allSetting.subIncyPerAppEnable}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subIncyPerAppEnable: v })}
                    options={onOff(t)}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyPerAppMode')}
                  description={t('pages.settings.subIncyPerAppModeDesc')}
                >
                  <Select
                    value={allSetting.subIncyPerAppMode}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subIncyPerAppMode: v })}
                    options={[
                      { value: '', label: t('pages.settings.subIncyNotSet') },
                      { value: 'proxy', label: t('pages.settings.subIncyPerAppModeProxy') },
                      { value: 'bypass', label: t('pages.settings.subIncyPerAppModeBypass') },
                    ]}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyPerAppList')}
                  description={t('pages.settings.subIncyPerAppListDesc')}
                >
                  <Input.TextArea
                    value={allSetting.subIncyPerAppList}
                    rows={4}
                    placeholder={
                      'com.google.chrome\norg.telegram.messenger\n\nor https://.../apps.txt'
                    }
                    onChange={(e) => updateSetting({ subIncyPerAppList: e.target.value })}
                  />
                </SettingListItem>
              </>
            ),
          },
          {
            key: 'network',
            label: catTabLabel(
              <ThunderboltOutlined />,
              t('pages.settings.subIncyGroupNetwork'),
              isMobile,
            ),
            children: (
              <>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyFragmentationEnable')}
                  description={t('pages.settings.subIncyFragmentationEnableDesc')}
                >
                  <Select
                    value={allSetting.subIncyFragmentationEnable}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subIncyFragmentationEnable: v })}
                    options={onOff(t)}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyFragmentLength')}
                  description={t('pages.settings.subIncyFragmentLengthDesc')}
                >
                  <Input
                    value={allSetting.subIncyFragmentLength}
                    placeholder="10-30"
                    onChange={(e) => updateSetting({ subIncyFragmentLength: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyFragmentInterval')}
                  description={t('pages.settings.subIncyFragmentIntervalDesc')}
                >
                  <Input
                    value={allSetting.subIncyFragmentInterval}
                    placeholder="20-40"
                    onChange={(e) => updateSetting({ subIncyFragmentInterval: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyFragmentPackets')}
                  description={t('pages.settings.subIncyFragmentPacketsDesc')}
                >
                  <Select
                    value={allSetting.subIncyFragmentPackets}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subIncyFragmentPackets: v })}
                    options={[
                      { value: '', label: t('pages.settings.subIncyNotSet') },
                      { value: 'tlshello', label: 'tlshello' },
                      { value: '1-3', label: '1-3' },
                      { value: '1', label: '1' },
                      { value: 'all', label: 'all' },
                    ]}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyNoisesEnable')}
                  description={t('pages.settings.subIncyNoisesEnableDesc')}
                >
                  <Select
                    value={allSetting.subIncyNoisesEnable}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subIncyNoisesEnable: v })}
                    options={onOff(t)}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyNoisesType')}
                  description={t('pages.settings.subIncyNoisesTypeDesc')}
                >
                  <Select
                    value={allSetting.subIncyNoisesType}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subIncyNoisesType: v })}
                    options={[
                      { value: '', label: t('pages.settings.subIncyNotSet') },
                      { value: 'rand', label: 'rand' },
                      { value: 'str', label: 'str' },
                      { value: 'hex', label: 'hex' },
                    ]}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyNoisesPacket')}
                  description={t('pages.settings.subIncyNoisesPacketDesc')}
                >
                  <Input
                    value={allSetting.subIncyNoisesPacket}
                    placeholder="10-20"
                    onChange={(e) => updateSetting({ subIncyNoisesPacket: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyNoisesDelay')}
                  description={t('pages.settings.subIncyNoisesDelayDesc')}
                >
                  <Input
                    value={allSetting.subIncyNoisesDelay}
                    placeholder="10-50"
                    onChange={(e) => updateSetting({ subIncyNoisesDelay: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyResolveEnable')}
                  description={t('pages.settings.subIncyResolveEnableDesc')}
                >
                  <Select
                    value={allSetting.subIncyResolveEnable}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subIncyResolveEnable: v })}
                    options={onOff(t)}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyResolveDnsDomain')}
                  description={t('pages.settings.subIncyResolveDnsDomainDesc')}
                >
                  <Input
                    value={allSetting.subIncyResolveDnsDomain}
                    placeholder="https://common.dot.dns.yandex.net/dns-query"
                    onChange={(e) => updateSetting({ subIncyResolveDnsDomain: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyResolveDnsIp')}
                  description={t('pages.settings.subIncyResolveDnsIpDesc')}
                >
                  <Input
                    value={allSetting.subIncyResolveDnsIp}
                    placeholder="77.88.8.8"
                    onChange={(e) => updateSetting({ subIncyResolveDnsIp: e.target.value })}
                  />
                </SettingListItem>
              </>
            ),
          },
          {
            key: 'routing',
            label: catTabLabel(
              <BranchesOutlined />,
              t('pages.settings.subIncyGroupRouting'),
              isMobile,
            ),
            children: (
              <>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyEnableRouting')}
                  description={t('pages.settings.subIncyEnableRoutingDesc')}
                >
                  <Switch
                    checked={allSetting.subIncyEnableRouting}
                    onChange={(v) => updateSetting({ subIncyEnableRouting: v })}
                  />
                </SettingListItem>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyRoutingRules')}
                  badge={remoteSourceBadge(allSetting.subIncyRoutingRules)}
                  description={t('pages.settings.subIncyRoutingRulesDesc')}
                >
                  <Input.TextArea
                    value={allSetting.subIncyRoutingRules}
                    placeholder="incy://routing/onadd/... or https://.../DEFAULT.JSON"
                    onChange={(e) => updateSetting({ subIncyRoutingRules: e.target.value })}
                  />
                </SettingListItem>
              </>
            ),
          },
        ]}
      />
    </>
  );
}
