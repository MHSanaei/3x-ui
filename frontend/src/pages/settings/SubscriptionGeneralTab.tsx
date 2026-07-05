import { Input, InputNumber, Switch, Tabs } from 'antd';
import { BranchesOutlined, CompassOutlined, IdcardOutlined, InfoCircleOutlined, NodeIndexOutlined, SafetyCertificateOutlined, SettingOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { AllSetting } from '@/models/setting';
import { SettingListItem } from '@/components/ui';
import { RemarkTemplateField } from '@/components/form';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from './catTabLabel';
import { sanitizePath, normalizePath } from './uriPath';

interface SubscriptionGeneralTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

export default function SubscriptionGeneralTab({ allSetting, updateSetting }: SubscriptionGeneralTabProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();

  return (
    <Tabs defaultActiveKey="1" items={[
      {
        key: '1',
        label: catTabLabel(<SettingOutlined />, t('pages.settings.panelSettings'), isMobile),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.subEnable')} description={t('pages.settings.subEnableDesc')}>
              <Switch checked={allSetting.subEnable} onChange={(v) => updateSetting({ subEnable: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subJsonEnableTitle')} description={t('pages.settings.subJsonEnable')}>
              <Switch checked={allSetting.subJsonEnable} onChange={(v) => updateSetting({ subJsonEnable: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subClashEnableTitle')}>
              <Switch checked={allSetting.subClashEnable} onChange={(v) => updateSetting({ subClashEnable: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subListen')} description={t('pages.settings.subListenDesc')}>
              <Input value={allSetting.subListen} onChange={(e) => updateSetting({ subListen: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subDomain')} description={t('pages.settings.subDomainDesc')}>
              <Input value={allSetting.subDomain} onChange={(e) => updateSetting({ subDomain: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subPort')} description={t('pages.settings.subPortDesc')}>
              <InputNumber value={allSetting.subPort} min={1} max={65535} style={{ width: '100%' }}
                onChange={(v) => updateSetting({ subPort: Number(v) || 0 })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subPath')} description={t('pages.settings.subPathDesc')}>
              <Input
                value={allSetting.subPath}
                placeholder="/sub/"
                onChange={(e) => updateSetting({ subPath: sanitizePath(e.target.value) })}
                onBlur={() => updateSetting({ subPath: normalizePath(allSetting.subPath) })}
              />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subURI')} description={t('pages.settings.subURIDesc')}>
              <Input value={allSetting.subURI} placeholder="(http|https)://domain[:port]/path/"
                onChange={(e) => updateSetting({ subURI: e.target.value })} />
            </SettingListItem>
          </>
        ),
      },
      {
        key: '2',
        label: catTabLabel(<InfoCircleOutlined />, t('pages.settings.information'), isMobile),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.subEncrypt')} description={t('pages.settings.subEncryptDesc')}>
              <Switch checked={allSetting.subEncrypt} onChange={(v) => updateSetting({ subEncrypt: v })} />
            </SettingListItem>
            <SettingListItem
              paddings="small"
              title={t('pages.settings.remarkTemplate')}
              description={t('pages.settings.remarkTemplateDesc')}
            >
              <RemarkTemplateField
                value={allSetting.remarkTemplate}
                onChange={(v) => updateSetting({ remarkTemplate: v })}
                maxLength={256}
              />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.subUpdates')} description={t('pages.settings.subUpdatesDesc')}>
              <InputNumber value={allSetting.subUpdates} min={0} max={525600} style={{ width: '100%' }}
                onChange={(v) => updateSetting({ subUpdates: Number(v) || 0 })} />
            </SettingListItem>
          </>
        ),
      },
      {
        key: '3',
        label: catTabLabel(<IdcardOutlined />, t('pages.settings.profile'), isMobile),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.subTitle')} description={t('pages.settings.subTitleDesc')}>
              <Input value={allSetting.subTitle} onChange={(e) => updateSetting({ subTitle: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subSupportUrl')} description={t('pages.settings.subSupportUrlDesc')}>
              <Input value={allSetting.subSupportUrl} placeholder="https://example.com"
                onChange={(e) => updateSetting({ subSupportUrl: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subProfileUrl')} description={t('pages.settings.subProfileUrlDesc')}>
              <Input value={allSetting.subProfileUrl} placeholder="https://example.com"
                onChange={(e) => updateSetting({ subProfileUrl: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subAnnounce')} description={t('pages.settings.subAnnounceDesc')}>
              <Input.TextArea value={allSetting.subAnnounce}
                onChange={(e) => updateSetting({ subAnnounce: e.target.value })} />
            </SettingListItem>
            <SettingListItem
              paddings="small"
              title={t('pages.settings.subThemeDir')}
              description={(
                <>
                  {t('pages.settings.subThemeDirDesc')}{' '}
                  <a
                    href="https://github.com/MHSanaei/3x-ui/blob/main/docs/custom-subscription-templates.md"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    {t('pages.settings.subThemeDirDocs')}
                  </a>
                </>
              )}
            >
              <Input value={allSetting.subThemeDir} placeholder="/etc/3x-ui/sub_templates/my-theme/"
                onChange={(e) => updateSetting({ subThemeDir: e.target.value })} />
            </SettingListItem>
          </>
        ),
      },
      {
        key: '4',
        label: catTabLabel(<SafetyCertificateOutlined />, t('pages.settings.certs'), isMobile),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.subCertPath')} description={t('pages.settings.subCertPathDesc')}>
              <Input value={allSetting.subCertFile} onChange={(e) => updateSetting({ subCertFile: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subKeyPath')} description={t('pages.settings.subKeyPathDesc')}>
              <Input value={allSetting.subKeyFile} onChange={(e) => updateSetting({ subKeyFile: e.target.value })} />
            </SettingListItem>
          </>
        ),
      },
      {
        key: '5',
        label: catTabLabel(<BranchesOutlined />, 'Happ', isMobile),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.subEnableRouting')} description={t('pages.settings.subEnableRoutingDesc')}>
              <Switch checked={allSetting.subEnableRouting} onChange={(v) => updateSetting({ subEnableRouting: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subRoutingRules')} description={t('pages.settings.subRoutingRulesDesc')}>
              <Input.TextArea value={allSetting.subRoutingRules} placeholder="happ://routing/add/..."
                onChange={(e) => updateSetting({ subRoutingRules: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subHideSettings')} description={t('pages.settings.subHideSettingsDesc')}>
              <Switch checked={allSetting.subHideSettings} onChange={(v) => updateSetting({ subHideSettings: v })} />
            </SettingListItem>
          </>
        ),
      },
      {
        key: '6',
        label: catTabLabel(<NodeIndexOutlined />, 'Clash / Mihomo', isMobile),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.subClashEnableRouting')} description={t('pages.settings.subClashEnableRoutingDesc')}>
              <Switch checked={allSetting.subClashEnableRouting} onChange={(v) => updateSetting({ subClashEnableRouting: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subClashRoutingRules')} description={t('pages.settings.subClashRoutingRulesDesc')}>
              <Input.TextArea
                value={allSetting.subClashRules}
                rows={8}
                placeholder={'GEOSITE,category-ir,DIRECT\nGEOIP,private,DIRECT'}
                onChange={(e) => updateSetting({ subClashRules: e.target.value })}
              />
            </SettingListItem>
          </>
        ),
      },
      {
        key: '7',
        label: catTabLabel(<CompassOutlined />, 'Incy', isMobile),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.subIncyEnableRouting')} description={t('pages.settings.subIncyEnableRoutingDesc')}>
              <Switch checked={allSetting.subIncyEnableRouting} onChange={(v) => updateSetting({ subIncyEnableRouting: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subIncyRoutingRules')} description={t('pages.settings.subIncyRoutingRulesDesc')}>
              <Input.TextArea value={allSetting.subIncyRoutingRules} placeholder="incy://routing/onadd/..."
                onChange={(e) => updateSetting({ subIncyRoutingRules: e.target.value })} />
            </SettingListItem>
          </>
        ),
      },
    ]} />
  );
}
