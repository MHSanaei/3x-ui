import { useMemo } from 'react';
import { Collapse, Divider, Input, InputNumber, Select, Space, Switch } from 'antd';
import { useTranslation } from 'react-i18next';
import type { AllSetting } from '@/models/setting';
import { SettingListItem } from '@/components/ui';
import { sanitizePath, normalizePath } from './uriPath';

const REMARK_MODELS: Record<string, string> = { i: 'Inbound', e: 'Email', o: 'Other' };
const REMARK_SAMPLES: Record<string, string> = { i: 'Germany', e: 'john', o: 'Relay' };
const REMARK_SEPARATORS = [' ', '-', '_', '@', ':', '~', '|', ',', '.', '/'];

interface SubscriptionGeneralTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

export default function SubscriptionGeneralTab({ allSetting, updateSetting }: SubscriptionGeneralTabProps) {
  const { t } = useTranslation();

  const remarkModel = useMemo(() => {
    const rm = allSetting.remarkModel || '';
    return rm.length > 1 ? rm.substring(1).split('') : [];
  }, [allSetting.remarkModel]);

  const remarkSeparator = useMemo(() => {
    const rm = allSetting.remarkModel || '-';
    return rm.length > 1 ? rm.charAt(0) : '-';
  }, [allSetting.remarkModel]);

  const remarkSample = useMemo(() => {
    const parts = remarkModel.map((k) => REMARK_SAMPLES[k]);
    return parts.length === 0 ? '' : parts.join(remarkSeparator);
  }, [remarkModel, remarkSeparator]);

  function setRemarkModel(parts: string[]) {
    updateSetting({ remarkModel: remarkSeparator + parts.join('') });
  }

  function setRemarkSeparator(sep: string) {
    const tail = (allSetting.remarkModel || '-').substring(1);
    updateSetting({ remarkModel: sep + tail });
  }

  return (
    <Collapse defaultActiveKey="1" items={[
      {
        key: '1',
        label: t('pages.settings.panelSettings'),
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
        label: t('pages.settings.information'),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.subEncrypt')} description={t('pages.settings.subEncryptDesc')}>
              <Switch checked={allSetting.subEncrypt} onChange={(v) => updateSetting({ subEncrypt: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subShowInfo')} description={t('pages.settings.subShowInfoDesc')}>
              <Switch checked={allSetting.subShowInfo} onChange={(v) => updateSetting({ subShowInfo: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subEmailInRemark')} description={t('pages.settings.subEmailInRemarkDesc')}>
              <Switch checked={allSetting.subEmailInRemark} onChange={(v) => updateSetting({ subEmailInRemark: v })} />
            </SettingListItem>

            <SettingListItem
              paddings="small"
              title={t('pages.settings.remarkModel')}
              description={
                <>
                  {t('pages.settings.sampleRemark')}:{' '}
                  <span
                    style={{
                      fontFamily: 'monospace',
                      padding: '1px 6px',
                      borderRadius: 4,
                      border: '1px solid var(--ant-color-border)',
                      background: 'var(--ant-color-fill-tertiary)',
                      whiteSpace: 'pre',
                    }}
                  >
                    {remarkSample ? `#${remarkSample}` : '—'}
                  </span>
                </>
              }
            >
              <Space.Compact style={{ width: '100%' }}>
                <Select
                  mode="multiple"
                  value={remarkModel}
                  onChange={setRemarkModel}
                  style={{ paddingRight: '.5rem', minWidth: '80%', width: 'auto' }}
                  options={Object.entries(REMARK_MODELS).map(([k, l]) => ({ value: k, label: l }))}
                />
                <Select
                  value={remarkSeparator}
                  onChange={setRemarkSeparator}
                  style={{ width: '20%' }}
                  options={REMARK_SEPARATORS.map((s) => ({ value: s, label: s === ' ' ? '␣' : s }))}
                />
              </Space.Compact>
            </SettingListItem>

            <Divider>{t('pages.settings.subTitle')}</Divider>

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

            <Divider>Happ</Divider>

            <SettingListItem paddings="small" title={t('pages.settings.subEnableRouting')} description={t('pages.settings.subEnableRoutingDesc')}>
              <Switch checked={allSetting.subEnableRouting} onChange={(v) => updateSetting({ subEnableRouting: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subRoutingRules')} description={t('pages.settings.subRoutingRulesDesc')}>
              <Input.TextArea value={allSetting.subRoutingRules} placeholder="happ://routing/add/..."
                onChange={(e) => updateSetting({ subRoutingRules: e.target.value })} />
            </SettingListItem>
          </>
        ),
      },
      {
        key: '3',
        label: t('pages.settings.certs'),
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
        key: '4',
        label: t('pages.settings.intervals'),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.subUpdates')} description={t('pages.settings.subUpdatesDesc')}>
              <InputNumber value={allSetting.subUpdates} min={1} style={{ width: '100%' }}
                onChange={(v) => updateSetting({ subUpdates: Number(v) || 0 })} />
            </SettingListItem>
          </>
        ),
      },
    ]} />
  );
}
