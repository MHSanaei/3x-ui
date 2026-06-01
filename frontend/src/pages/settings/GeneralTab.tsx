import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Collapse,
  Input,
  InputNumber,
  Select,
  Space,
  Switch,
} from 'antd';
import type { AllSetting } from '@/models/setting';
import { HttpUtil, LanguageManager } from '@/utils';
import { SettingListItem } from '@/components/ui';
import { sanitizePath } from './uriPath';

interface ApiMsg<T = unknown> {
  success?: boolean;
  obj?: T;
}

interface GeneralTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

const REMARK_MODELS: Record<string, string> = { i: 'Inbound', e: 'Email', o: 'Other' };
const REMARK_SEPARATORS = [' ', '-', '_', '@', ':', '~', '|', ',', '.', '/'];
const DATEPICKER_LIST: { name: string; value: 'gregorian' | 'jalalian' }[] = [
  { name: 'Gregorian (Standard)', value: 'gregorian' },
  { name: 'Jalalian (شمسی)', value: 'jalalian' },
];

export default function GeneralTab({ allSetting, updateSetting }: GeneralTabProps) {
  const { t } = useTranslation();

  const [lang, setLang] = useState<string>(() => LanguageManager.getLanguage());
  const [inboundOptions, setInboundOptions] = useState<{ label: string; value: string }[]>([]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      // /options is the slim picker-shaped endpoint — it skips the heavy
      // per-client settings and clientStats payloads that /list ships.
      const msg = await HttpUtil.get('/panel/api/inbounds/options') as ApiMsg<{
        tag: string; protocol: string; port: number;
      }[]>;
      if (cancelled) return;
      if (msg?.success && Array.isArray(msg.obj)) {
        setInboundOptions(msg.obj.map((ib) => ({
          label: `${ib.tag} (${ib.protocol}@${ib.port})`,
          value: ib.tag,
        })));
      } else {
        setInboundOptions([]);
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const remarkModel = useMemo(() => {
    const rm = allSetting.remarkModel || '';
    return rm.length > 1 ? rm.substring(1).split('') : [];
  }, [allSetting.remarkModel]);

  const remarkSeparator = useMemo(() => {
    const rm = allSetting.remarkModel || '-';
    return rm.length > 1 ? rm.charAt(0) : '-';
  }, [allSetting.remarkModel]);

  const remarkSample = useMemo(() => {
    const parts = remarkModel.map((k) => REMARK_MODELS[k]);
    return parts.length === 0 ? '' : parts.join(remarkSeparator);
  }, [remarkModel, remarkSeparator]);

  function setRemarkModel(parts: string[]) {
    updateSetting({ remarkModel: remarkSeparator + parts.join('') });
  }

  function setRemarkSeparator(sep: string) {
    const tail = (allSetting.remarkModel || '-').substring(1);
    updateSetting({ remarkModel: sep + tail });
  }

  const ldapInboundTagList = useMemo(() => {
    const csv = allSetting.ldapInboundTags || '';
    return csv.length ? csv.split(',').map((s) => s.trim()).filter(Boolean) : [];
  }, [allSetting.ldapInboundTags]);

  function setLdapInboundTagList(list: string[]) {
    updateSetting({ ldapInboundTags: Array.isArray(list) ? list.join(',') : '' });
  }

  function onLangChange(value: string) {
    setLang(value);
    LanguageManager.setLanguage(value);
  }

  const langOptions = useMemo(
    () => LanguageManager.supportedLanguages.map((l: { value: string; name: string; icon: string }) => ({
      value: l.value,
      label: (
        <>
          <span role="img" aria-label={l.name}>{l.icon}</span>
          &nbsp;&nbsp;<span>{l.name}</span>
        </>
      ),
    })),
    [],
  );

  return (
    <Collapse defaultActiveKey="1" items={[
      {
        key: '1',
        label: t('pages.settings.panelSettings'),
        children: (
          <>
            <SettingListItem
              paddings="small"
              title={t('pages.settings.remarkModel')}
              description={<>{t('pages.settings.sampleRemark')}: <i>#{remarkSample}</i></>}
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
                  options={REMARK_SEPARATORS.map((s) => ({ value: s, label: s }))}
                />
              </Space.Compact>
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.panelListeningIP')} description={t('pages.settings.panelListeningIPDesc')}>
              <Input value={allSetting.webListen} onChange={(e) => updateSetting({ webListen: e.target.value })} />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.panelListeningDomain')} description={t('pages.settings.panelListeningDomainDesc')}>
              <Input value={allSetting.webDomain} onChange={(e) => updateSetting({ webDomain: e.target.value })} />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.panelPort')} description={t('pages.settings.panelPortDesc')}>
              <InputNumber value={allSetting.webPort} min={1} max={65535} style={{ width: '100%' }}
                onChange={(v) => updateSetting({ webPort: Number(v) || 0 })} />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.panelUrlPath')} description={t('pages.settings.panelUrlPathDesc')}>
              <Input value={allSetting.webBasePath} onChange={(e) => updateSetting({ webBasePath: sanitizePath(e.target.value) })} />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.sessionMaxAge')} description={t('pages.settings.sessionMaxAgeDesc')}>
              <InputNumber value={allSetting.sessionMaxAge} min={60} max={525600} style={{ width: '100%' }}
                onChange={(v) => updateSetting({ sessionMaxAge: Number(v) || 0 })} />
            </SettingListItem>

            <SettingListItem
              paddings="small"
              title={t('pages.settings.trustedProxyCidrs')}
              description={t('pages.settings.trustedProxyCidrsDesc')}
            >
              <Input
                value={allSetting.trustedProxyCIDRs}
                placeholder="127.0.0.1/32,::1/128"
                onChange={(e) => updateSetting({ trustedProxyCIDRs: e.target.value })}
              />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.panelProxy')} description={t('pages.settings.panelProxyDesc')}>
              <Input
                value={allSetting.panelProxy}
                placeholder="socks5:// or http://user:pass@host:port"
                onChange={(e) => updateSetting({ panelProxy: e.target.value })}
              />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.pageSize')} description={t('pages.settings.pageSizeDesc')}>
              <InputNumber value={allSetting.pageSize} min={1} max={1000} step={5} style={{ width: '100%' }}
                onChange={(v) => updateSetting({ pageSize: Number(v) || 0 })} />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.language')}>
              <Select
                value={lang}
                onChange={onLangChange}
                style={{ width: '100%' }}
                options={langOptions}
              />
            </SettingListItem>
          </>
        ),
      },
      {
        key: '2',
        label: t('pages.settings.notifications'),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.expireTimeDiff')} description={t('pages.settings.expireTimeDiffDesc')}>
              <InputNumber value={allSetting.expireDiff} min={0} style={{ width: '100%' }}
                onChange={(v) => updateSetting({ expireDiff: Number(v) || 0 })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.trafficDiff')} description={t('pages.settings.trafficDiffDesc')}>
              <InputNumber value={allSetting.trafficDiff} min={0} style={{ width: '100%' }}
                onChange={(v) => updateSetting({ trafficDiff: Number(v) || 0 })} />
            </SettingListItem>
          </>
        ),
      },
      {
        key: '3',
        label: t('pages.settings.certs'),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.publicKeyPath')} description={t('pages.settings.publicKeyPathDesc')}>
              <Input value={allSetting.webCertFile} onChange={(e) => updateSetting({ webCertFile: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.privateKeyPath')} description={t('pages.settings.privateKeyPathDesc')}>
              <Input value={allSetting.webKeyFile} onChange={(e) => updateSetting({ webKeyFile: e.target.value })} />
            </SettingListItem>
          </>
        ),
      },
      {
        key: '4',
        label: t('pages.settings.externalTraffic'),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.externalTrafficInformEnable')} description={t('pages.settings.externalTrafficInformEnableDesc')}>
              <Switch checked={allSetting.externalTrafficInformEnable}
                onChange={(v) => updateSetting({ externalTrafficInformEnable: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.externalTrafficInformURI')} description={t('pages.settings.externalTrafficInformURIDesc')}>
              <Input
                value={allSetting.externalTrafficInformURI}
                placeholder="(http|https)://domain[:port]/path/"
                onChange={(e) => updateSetting({ externalTrafficInformURI: e.target.value })}
              />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.restartXrayOnClientDisable')} description={t('pages.settings.restartXrayOnClientDisableDesc')}>
              <Switch checked={allSetting.restartXrayOnClientDisable}
                onChange={(v) => updateSetting({ restartXrayOnClientDisable: v })} />
            </SettingListItem>
          </>
        ),
      },
      {
        key: '5',
        label: t('pages.settings.dateAndTime'),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.timeZone')} description={t('pages.settings.timeZoneDesc')}>
              <Input value={allSetting.timeLocation} onChange={(e) => updateSetting({ timeLocation: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.datepicker')} description={t('pages.settings.datepickerDescription')}>
              <Select
                value={allSetting.datepicker || 'gregorian'}
                onChange={(v) => updateSetting({ datepicker: v as 'gregorian' | 'jalalian' })}
                style={{ width: '100%' }}
                options={DATEPICKER_LIST.map((d) => ({ value: d.value, label: d.name }))}
              />
            </SettingListItem>
          </>
        ),
      },
      {
        key: '6',
        label: 'LDAP',
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.enable')}>
              <Switch checked={allSetting.ldapEnable} onChange={(v) => updateSetting({ ldapEnable: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.host')}>
              <Input value={allSetting.ldapHost} onChange={(e) => updateSetting({ ldapHost: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.port')}>
              <InputNumber value={allSetting.ldapPort} min={1} max={65535} style={{ width: '100%' }}
                onChange={(v) => updateSetting({ ldapPort: Number(v) || 0 })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.useTls')}>
              <Switch checked={allSetting.ldapUseTLS} onChange={(v) => updateSetting({ ldapUseTLS: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.bindDn')}>
              <Input value={allSetting.ldapBindDN} onChange={(e) => updateSetting({ ldapBindDN: e.target.value })} />
            </SettingListItem>
            <SettingListItem
              paddings="small"
              title={t('password')}
              description={allSetting.hasLdapPassword ? t('pages.settings.ldap.passwordConfigured') : t('pages.settings.ldap.passwordUnconfigured')}
            >
              <Input.Password
                value={allSetting.ldapPassword}
                placeholder={allSetting.hasLdapPassword ? t('pages.settings.ldap.passwordPlaceholder') : ''}
                onChange={(e) => updateSetting({ ldapPassword: e.target.value })}
              />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.baseDn')}>
              <Input value={allSetting.ldapBaseDN} onChange={(e) => updateSetting({ ldapBaseDN: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.userFilter')}>
              <Input value={allSetting.ldapUserFilter} onChange={(e) => updateSetting({ ldapUserFilter: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.userAttr')}>
              <Input value={allSetting.ldapUserAttr} onChange={(e) => updateSetting({ ldapUserAttr: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.vlessField')}>
              <Input value={allSetting.ldapVlessField} onChange={(e) => updateSetting({ ldapVlessField: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.flagField')} description={t('pages.settings.ldap.flagFieldDesc')}>
              <Input value={allSetting.ldapFlagField} onChange={(e) => updateSetting({ ldapFlagField: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.truthyValues')} description={t('pages.settings.ldap.truthyValuesDesc')}>
              <Input value={allSetting.ldapTruthyValues} onChange={(e) => updateSetting({ ldapTruthyValues: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.invertFlag')} description={t('pages.settings.ldap.invertFlagDesc')}>
              <Switch checked={allSetting.ldapInvertFlag} onChange={(v) => updateSetting({ ldapInvertFlag: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.syncSchedule')} description={t('pages.settings.ldap.syncScheduleDesc')}>
              <Input value={allSetting.ldapSyncCron} onChange={(e) => updateSetting({ ldapSyncCron: e.target.value })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.inboundTags')} description={t('pages.settings.ldap.inboundTagsDesc')}>
              <>
                <Select
                  mode="multiple"
                  value={ldapInboundTagList}
                  onChange={setLdapInboundTagList}
                  style={{ width: '100%' }}
                  options={inboundOptions}
                />
                {inboundOptions.length === 0 && (
                  <div className="ldap-no-inbounds">{t('pages.settings.ldap.noInbounds')}</div>
                )}
              </>
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.autoCreate')}>
              <Switch checked={allSetting.ldapAutoCreate} onChange={(v) => updateSetting({ ldapAutoCreate: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.autoDelete')}>
              <Switch checked={allSetting.ldapAutoDelete} onChange={(v) => updateSetting({ ldapAutoDelete: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.defaultTotalGb')}>
              <InputNumber value={allSetting.ldapDefaultTotalGB} min={0} style={{ width: '100%' }}
                onChange={(v) => updateSetting({ ldapDefaultTotalGB: Number(v) || 0 })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.defaultExpiryDays')}>
              <InputNumber value={allSetting.ldapDefaultExpiryDays} min={0} style={{ width: '100%' }}
                onChange={(v) => updateSetting({ ldapDefaultExpiryDays: Number(v) || 0 })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.ldap.defaultIpLimit')}>
              <InputNumber value={allSetting.ldapDefaultLimitIP} min={0} style={{ width: '100%' }}
                onChange={(v) => updateSetting({ ldapDefaultLimitIP: Number(v) || 0 })} />
            </SettingListItem>
          </>
        ),
      },
    ]} />
  );
}
