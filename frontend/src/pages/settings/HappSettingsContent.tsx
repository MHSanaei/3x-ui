import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Input, Modal, Select, Space, Switch, Tabs, message } from 'antd';
import {
  BranchesOutlined,
  BuildOutlined,
  CloudSyncOutlined,
  DesktopOutlined,
  LinkOutlined,
  MobileOutlined,
  NotificationOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import type { AllSetting } from '@/models/setting';
import { SettingListItem } from '@/components/ui';
import { buildHappPresetDeeplink, parseList, toBase64Utf8 } from './happPresets';
import { catTabLabel } from './catTabLabel';

interface HappSettingsContentProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
  isMobile: boolean;
  remoteSourceBadge: (val: string) => React.ReactNode;
  defaultActiveTab?: 'routing' | 'links';
}

export default function HappSettingsContent({
  allSetting,
  updateSetting,
  isMobile,
  remoteSourceBadge,
  defaultActiveTab = 'routing',
}: HappSettingsContentProps) {
  const { t } = useTranslation();
  const [selectedPreset, setSelectedPreset] = useState<string>('iran-bypass');
  const [isModalOpen, setIsModalOpen] = useState(false);

  const [directDomains, setDirectDomains] = useState('');
  const [proxyDomains, setProxyDomains] = useState('');
  const [blockDomains, setBlockDomains] = useState('');
  const [directIPs, setDirectIPs] = useState('');
  const [proxyIPs, setProxyIPs] = useState('');
  const [blockIPs, setBlockIPs] = useState('');

  const applyPreset = () => {
    const payload = buildHappPresetDeeplink(selectedPreset);
    if (payload) {
      updateSetting({ subRoutingRules: payload });
      message.success(t('pages.settings.subHappPresetApplied'));
    }
  };

  const handleBuildDeeplink = () => {
    interface FieldRule {
      type: string;
      outboundTag: string;
      domain?: string[];
      ip?: string[];
      network?: string;
    }
    const rules: FieldRule[] = [];

    const bDom = parseList(blockDomains);
    const bIp = parseList(blockIPs);
    if (bDom.length > 0 || bIp.length > 0) {
      rules.push({
        type: 'field',
        outboundTag: 'block',
        ...(bDom.length > 0 ? { domain: bDom } : {}),
        ...(bIp.length > 0 ? { ip: bIp } : {}),
      });
    }

    const dDom = parseList(directDomains);
    const dIp = parseList(directIPs);
    if (dDom.length > 0 || dIp.length > 0) {
      rules.push({
        type: 'field',
        outboundTag: 'direct',
        ...(dDom.length > 0 ? { domain: dDom } : {}),
        ...(dIp.length > 0 ? { ip: dIp } : {}),
      });
    }

    const pDom = parseList(proxyDomains);
    const pIp = parseList(proxyIPs);
    if (pDom.length > 0 || pIp.length > 0) {
      rules.push({
        type: 'field',
        outboundTag: 'proxy',
        ...(pDom.length > 0 ? { domain: pDom } : {}),
        ...(pIp.length > 0 ? { ip: pIp } : {}),
      });
    }

    rules.push({
      type: 'field',
      outboundTag: 'proxy',
      network: 'tcp,udp',
    });

    const deeplink = 'happ://routing/onadd/' + toBase64Utf8(JSON.stringify({ rules }));
    updateSetting({ subRoutingRules: deeplink });
    setIsModalOpen(false);
    message.success(t('pages.settings.subHappDeeplinkGenerated'));
  };

  return (
    <>
      <Tabs
        type="card"
        size="small"
        defaultActiveKey={defaultActiveTab}
        items={[
          {
            key: 'routing',
            label: (
              <span>
                <BranchesOutlined /> {!isMobile && t('pages.settings.subHappGroupRouting')}
              </span>
            ),
            children: (
              <>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subEnableRouting')}
                  description={t('pages.settings.subEnableRoutingDesc')}
                >
                  <Switch
                    checked={allSetting.subEnableRouting}
                    onChange={(v) => updateSetting({ subEnableRouting: v })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappPresets')}
                  description={t('pages.settings.subHappPresetsDesc')}
                >
                  <Space orientation="horizontal" style={{ width: '100%' }}>
                    <Select
                      value={selectedPreset}
                      style={{ minWidth: 170 }}
                      onChange={setSelectedPreset}
                      options={[
                        { value: 'iran-bypass', label: t('pages.settings.subHappPresetIran') },
                        { value: 'china-direct', label: t('pages.settings.subHappPresetChina') },
                        { value: 'adblock', label: t('pages.settings.subHappPresetAdblock') },
                        { value: 'global', label: t('pages.settings.subHappPresetGlobal') },
                        { value: 'off', label: t('pages.settings.subHappPresetOff') },
                      ]}
                    />
                    <Button type="primary" onClick={applyPreset}>
                      {t('pages.settings.subHappPresets')}
                    </Button>
                  </Space>
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappVisualBuilder')}
                  description={t('pages.settings.subHappVisualBuilderDesc')}
                >
                  <Button icon={<BuildOutlined />} onClick={() => setIsModalOpen(true)}>
                    {t('pages.settings.subHappVisualBuilder')}
                  </Button>
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subRoutingRules')}
                  badge={remoteSourceBadge(allSetting.subRoutingRules)}
                  description={t('pages.settings.subRoutingRulesDesc')}
                >
                  <Input.TextArea
                    value={allSetting.subRoutingRules}
                    rows={4}
                    placeholder="happ://routing/onadd/... or https://.../DEFAULT.DEEPLINK"
                    onChange={(e) => updateSetting({ subRoutingRules: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappNoLimit')}
                  description={t('pages.settings.subHappNoLimitDesc')}
                >
                  <Switch
                    checked={allSetting.subHappNoLimit}
                    onChange={(v) => updateSetting({ subHappNoLimit: v })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHideSettings')}
                  description={t('pages.settings.subHideSettingsDesc')}
                >
                  <Switch
                    checked={allSetting.subHideSettings}
                    onChange={(v) => updateSetting({ subHideSettings: v })}
                  />
                </SettingListItem>
              </>
            ),
          },
          {
            key: 'links',
            label: catTabLabel(<LinkOutlined />, t('pages.settings.subHappGroupLinks'), isMobile),
            children: (
              <SettingListItem
                paddings="small"
                title={t('pages.settings.happLinkEnable')}
                description={t('pages.settings.happLinkEnableDesc')}
              >
                <Switch
                  checked={allSetting.happLinkEnable}
                  onChange={(v) => updateSetting({ happLinkEnable: v })}
                />
              </SettingListItem>
            ),
          },
          {
            key: 'banners',
            label: (
              <span>
                <NotificationOutlined /> {!isMobile && t('pages.settings.subHappGroupBanners')}
              </span>
            ),
            children: (
              <>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappSubInfoText')}
                  description={t('pages.settings.subHappSubInfoTextDesc')}
                >
                  <Input
                    value={allSetting.subHappSubInfoText}
                    maxLength={200}
                    placeholder="Welcome to our high-speed network!"
                    onChange={(e) => updateSetting({ subHappSubInfoText: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappSubInfoColor')}
                  description={t('pages.settings.subHappSubInfoColorDesc')}
                >
                  <Select
                    value={allSetting.subHappSubInfoColor || 'blue'}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subHappSubInfoColor: v })}
                    options={[
                      { value: 'blue', label: t('pages.settings.subHappColorBlue') },
                      { value: 'green', label: t('pages.settings.subHappColorGreen') },
                      { value: 'red', label: t('pages.settings.subHappColorRed') },
                    ]}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappSubInfoButtonText')}
                  description={t('pages.settings.subHappSubInfoButtonTextDesc')}
                >
                  <Input
                    value={allSetting.subHappSubInfoButtonText}
                    maxLength={25}
                    placeholder="Support Channel"
                    onChange={(e) => updateSetting({ subHappSubInfoButtonText: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappSubInfoButtonLink')}
                  description={t('pages.settings.subHappSubInfoButtonLinkDesc')}
                >
                  <Input
                    value={allSetting.subHappSubInfoButtonLink}
                    placeholder="https://t.me/your_channel"
                    onChange={(e) => updateSetting({ subHappSubInfoButtonLink: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappSubExpire')}
                  description={t('pages.settings.subHappSubExpireDesc')}
                >
                  <Switch
                    checked={allSetting.subHappSubExpire}
                    onChange={(v) => updateSetting({ subHappSubExpire: v })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappSubExpireButtonLink')}
                  description={t('pages.settings.subHappSubExpireButtonLinkDesc')}
                >
                  <Input
                    value={allSetting.subHappSubExpireButtonLink}
                    placeholder="https://example.com/renew"
                    onChange={(e) => updateSetting({ subHappSubExpireButtonLink: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappNotificationExpire')}
                  description={t('pages.settings.subHappNotificationExpireDesc')}
                >
                  <Switch
                    checked={allSetting.subHappNotificationExpire}
                    onChange={(v) => updateSetting({ subHappNotificationExpire: v })}
                  />
                </SettingListItem>
              </>
            ),
          },
          {
            key: 'network',
            label: (
              <span>
                <ThunderboltOutlined /> {!isMobile && t('pages.settings.subHappGroupNetwork')}
              </span>
            ),
            children: (
              <>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappTunMode')}
                  description={t('pages.settings.subHappTunModeDesc')}
                >
                  <Select
                    value={allSetting.subHappTunMode}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subHappTunMode: v })}
                    options={[
                      // happ.su documents tun-mode as system|gvisor only, so
                      // Default is the unset state rather than a third value.
                      { value: '', label: t('pages.settings.subHappTunModeDefault') },
                      { value: 'system', label: t('pages.settings.subHappTunModeSystem') },
                      { value: 'gvisor', label: t('pages.settings.subHappTunModeGvisor') },
                    ]}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappTunType')}
                  description={t('pages.settings.subHappTunTypeDesc')}
                >
                  <Select
                    value={allSetting.subHappTunType || 'default'}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subHappTunType: v })}
                    options={[
                      { value: 'singbox', label: t('pages.settings.subHappTunTypeSingbox') },
                      { value: 'tun2proxy', label: t('pages.settings.subHappTunTypeTun2proxy') },
                      { value: 'default', label: t('pages.settings.subHappTunTypeDefault') },
                      { value: 'xray', label: t('pages.settings.subHappTunTypeXray') },
                    ]}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappExcludeRoutes')}
                  description={t('pages.settings.subHappExcludeRoutesDesc')}
                >
                  <Input
                    value={allSetting.subHappExcludeRoutes}
                    placeholder="192.168.0.0/16, 10.0.0.0/8"
                    onChange={(e) => updateSetting({ subHappExcludeRoutes: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappExcludeApns')}
                  description={t('pages.settings.subHappExcludeApnsDesc')}
                >
                  <Switch
                    checked={allSetting.subHappExcludeApns}
                    onChange={(v) => updateSetting({ subHappExcludeApns: v })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappPingType')}
                  description={t('pages.settings.subHappPingTypeDesc')}
                >
                  <Select
                    value={allSetting.subHappPingType || 'proxy'}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subHappPingType: v })}
                    options={[
                      { value: 'proxy', label: t('pages.settings.subHappPingProxy') },
                      { value: 'proxy-head', label: t('pages.settings.subHappPingProxyHead') },
                      { value: 'tcp', label: t('pages.settings.subHappPingTcp') },
                      { value: 'icmp', label: t('pages.settings.subHappPingIcmp') },
                    ]}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappAutoConnect')}
                  description={t('pages.settings.subHappAutoConnectDesc')}
                >
                  <Switch
                    checked={allSetting.subHappAutoConnect}
                    onChange={(v) => updateSetting({ subHappAutoConnect: v })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappAutoConnectType')}
                  description={t('pages.settings.subHappAutoConnectTypeDesc')}
                >
                  <Select
                    value={allSetting.subHappAutoConnectType || 'lowestdelay'}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subHappAutoConnectType: v })}
                    options={[
                      {
                        value: 'lowestdelay',
                        label: t('pages.settings.subHappAutoConnectLowestDelay'),
                      },
                      { value: 'lastused', label: t('pages.settings.subHappAutoConnectLastUsed') },
                      { value: 'random', label: t('pages.settings.subHappAutoConnectRandom') },
                    ]}
                  />
                </SettingListItem>
              </>
            ),
          },
          {
            key: 'themes',
            label: (
              <span>
                <DesktopOutlined /> {!isMobile && t('pages.settings.subHappGroupThemes')}
              </span>
            ),
            children: (
              <>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappColorProfile')}
                  description={t('pages.settings.subHappColorProfileDesc')}
                >
                  <Input
                    value={allSetting.subHappColorProfile}
                    placeholder="default, violet, turquoise, cyberpunk, or custom JSON"
                    onChange={(e) => updateSetting({ subHappColorProfile: e.target.value })}
                  />
                </SettingListItem>
              </>
            ),
          },
          {
            key: 'failover',
            label: (
              <span>
                <CloudSyncOutlined /> {!isMobile && t('pages.settings.subHappGroupFailover')}
              </span>
            ),
            children: (
              <>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappAutoDetect')}
                  description={t('pages.settings.subHappAutoDetectDesc')}
                >
                  <Switch
                    checked={allSetting.subHappAutoDetect}
                    onChange={(v) => updateSetting({ subHappAutoDetect: v })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappProviderId')}
                  description={t('pages.settings.subHappProviderIdDesc')}
                >
                  <Input
                    value={allSetting.subHappProviderId}
                    placeholder="my-happ-provider"
                    onChange={(e) => updateSetting({ subHappProviderId: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappNewUrl')}
                  description={t('pages.settings.subHappNewUrlDesc')}
                >
                  <Input
                    value={allSetting.subHappNewUrl}
                    placeholder="https://new-domain.com/sub/..."
                    onChange={(e) => updateSetting({ subHappNewUrl: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappFallbackUrl')}
                  description={t('pages.settings.subHappFallbackUrlDesc')}
                >
                  <Input
                    value={allSetting.subHappFallbackUrl}
                    placeholder="https://backup-domain.com/sub/..."
                    onChange={(e) => updateSetting({ subHappFallbackUrl: e.target.value })}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappAlwaysHwid')}
                  description={t('pages.settings.subHappAlwaysHwidDesc')}
                >
                  <Switch
                    checked={allSetting.subHappAlwaysHwid}
                    onChange={(v) => updateSetting({ subHappAlwaysHwid: v })}
                  />
                </SettingListItem>
              </>
            ),
          },
          {
            key: 'android',
            label: (
              <span>
                <MobileOutlined /> {!isMobile && t('pages.settings.subHappGroupAndroid')}
              </span>
            ),
            children: (
              <>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappPerAppMode')}
                  description={t('pages.settings.subHappPerAppModeDesc')}
                >
                  <Select
                    value={allSetting.subHappPerAppMode || 'off'}
                    style={{ width: '100%' }}
                    onChange={(v) => updateSetting({ subHappPerAppMode: v })}
                    options={[
                      { value: 'off', label: t('pages.settings.subHappPerAppOff') },
                      { value: 'on', label: t('pages.settings.subHappPerAppOn') },
                      { value: 'bypass', label: t('pages.settings.subHappPerAppBypass') },
                    ]}
                  />
                </SettingListItem>

                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subHappPerAppList')}
                  description={t('pages.settings.subHappPerAppListDesc')}
                >
                  <Input.TextArea
                    value={allSetting.subHappPerAppList}
                    rows={4}
                    placeholder="org.telegram.messenger, com.google.android.youtube"
                    onChange={(e) => updateSetting({ subHappPerAppList: e.target.value })}
                  />
                </SettingListItem>
              </>
            ),
          },
        ]}
      />

      <Modal
        title={t('pages.settings.subHappModalTitle')}
        open={isModalOpen}
        onCancel={() => setIsModalOpen(false)}
        onOk={handleBuildDeeplink}
        okText={t('pages.settings.subHappBuildDeeplink')}
        width={650}
      >
        <Space orientation="vertical" style={{ width: '100%', marginTop: 12 }} size="middle">
          <div>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>
              {t('pages.settings.subHappDirectDomains')}
            </div>
            <Input.TextArea
              rows={2}
              value={directDomains}
              placeholder="domain:ir, domain:cn, example.local"
              onChange={(e) => setDirectDomains(e.target.value)}
            />
          </div>
          <div>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>
              {t('pages.settings.subHappProxyDomains')}
            </div>
            <Input.TextArea
              rows={2}
              value={proxyDomains}
              placeholder="geosite:google, youtube.com"
              onChange={(e) => setProxyDomains(e.target.value)}
            />
          </div>
          <div>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>
              {t('pages.settings.subHappBlockDomains')}
            </div>
            <Input.TextArea
              rows={2}
              value={blockDomains}
              placeholder="geosite:category-ads-all, analytics.google.com"
              onChange={(e) => setBlockDomains(e.target.value)}
            />
          </div>
          <div>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>
              {t('pages.settings.subHappDirectIPs')}
            </div>
            <Input.TextArea
              rows={2}
              value={directIPs}
              placeholder="geoip:ir, 192.168.0.0/16, 10.0.0.0/8"
              onChange={(e) => setDirectIPs(e.target.value)}
            />
          </div>
          <div>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>
              {t('pages.settings.subHappProxyIPs')}
            </div>
            <Input.TextArea
              rows={2}
              value={proxyIPs}
              placeholder="1.1.1.1/32, 8.8.8.8/32"
              onChange={(e) => setProxyIPs(e.target.value)}
            />
          </div>
          <div>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>
              {t('pages.settings.subHappBlockIPs')}
            </div>
            <Input.TextArea
              rows={2}
              value={blockIPs}
              placeholder="geoip:phishing, 0.0.0.0/8"
              onChange={(e) => setBlockIPs(e.target.value)}
            />
          </div>
        </Space>
      </Modal>
    </>
  );
}
