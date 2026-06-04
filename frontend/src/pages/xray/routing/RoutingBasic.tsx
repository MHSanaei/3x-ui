import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Select, Switch } from 'antd';

import { SettingListItem } from '@/components/ui';
import type { XraySettingsValue, SetTemplate } from '@/hooks/useXraySetting';
import {
  BITTORRENT_PROTOCOLS,
  BLOCK_DOMAINS_OPTIONS,
  DOMAINS_OPTIONS,
  IPS_OPTIONS,
  SERVICES_OPTIONS,
  directSettings,
  ipv4Settings,
} from '../basics/constants';
import { ruleGetter, ruleSetter, syncOutbound } from '../basics/helpers';

interface RoutingBasicProps {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
}

export default function RoutingBasic({ templateSettings, setTemplateSettings }: RoutingBasicProps) {
  const { t } = useTranslation();

  const mutate = useCallback(
    (mutator: (next: XraySettingsValue) => void) => {
      setTemplateSettings((prev) => {
        if (!prev) return prev;
        const clone = JSON.parse(JSON.stringify(prev)) as XraySettingsValue;
        mutator(clone);
        return clone;
      });
    },
    [setTemplateSettings],
  );

  const blockedIPs = ruleGetter(templateSettings, 'blocked', 'ip');
  const blockedDomains = ruleGetter(templateSettings, 'blocked', 'domain');
  const blockedProtocols = ruleGetter(templateSettings, 'blocked', 'protocol');
  const directIPs = ruleGetter(templateSettings, 'direct', 'ip');
  const directDomains = ruleGetter(templateSettings, 'direct', 'domain');
  const ipv4Domains = ruleGetter(templateSettings, 'IPv4', 'domain');

  const torrentActive = BITTORRENT_PROTOCOLS.every((p) => blockedProtocols.includes(p));

  return (
    <>
      <Alert
        type="warning"
        showIcon
        className="mb-12 hint-alert"
        title={t('pages.xray.blockConnectionsConfigsDesc')}
      />

      <SettingListItem
        title={t('pages.xray.Torrent')}
        paddings="small"
        control={
          <Switch
            checked={torrentActive}
            onChange={(checked) => mutate((tt) => {
              const next = checked
                ? [...blockedProtocols, ...BITTORRENT_PROTOCOLS]
                : blockedProtocols.filter((d) => !BITTORRENT_PROTOCOLS.includes(d));
              ruleSetter(tt, 'blocked', 'protocol', next);
            })}
          />
        }
      />

      <SettingListItem
        title={t('pages.xray.blockips')}
        paddings="small"
        control={
          <Select
            mode="tags"
            value={blockedIPs}
            style={{ width: '100%' }}
            options={IPS_OPTIONS}
            onChange={(v) => mutate((tt) => ruleSetter(tt, 'blocked', 'ip', v))}
          />
        }
      />

      <SettingListItem
        title={t('pages.xray.blockdomains')}
        paddings="small"
        control={
          <Select
            mode="tags"
            value={blockedDomains}
            style={{ width: '100%' }}
            options={BLOCK_DOMAINS_OPTIONS}
            onChange={(v) => mutate((tt) => ruleSetter(tt, 'blocked', 'domain', v))}
          />
        }
      />

      <Alert
        type="warning"
        showIcon
        className="mb-12 hint-alert"
        title={t('pages.xray.directConnectionsConfigsDesc')}
      />

      <SettingListItem
        title={t('pages.xray.directips')}
        paddings="small"
        control={
          <Select
            mode="tags"
            value={directIPs}
            style={{ width: '100%' }}
            options={IPS_OPTIONS}
            onChange={(v) => mutate((tt) => {
              ruleSetter(tt, 'direct', 'ip', v);
              syncOutbound(tt, 'direct', directSettings);
            })}
          />
        }
      />

      <SettingListItem
        title={t('pages.xray.directdomains')}
        paddings="small"
        control={
          <Select
            mode="tags"
            value={directDomains}
            style={{ width: '100%' }}
            options={DOMAINS_OPTIONS}
            onChange={(v) => mutate((tt) => {
              ruleSetter(tt, 'direct', 'domain', v);
              syncOutbound(tt, 'direct', directSettings);
            })}
          />
        }
      />

      <SettingListItem
        title={t('pages.xray.ipv4Routing')}
        description={t('pages.xray.ipv4RoutingDesc')}
        paddings="small"
        control={
          <Select
            mode="tags"
            value={ipv4Domains}
            style={{ width: '100%' }}
            options={SERVICES_OPTIONS}
            onChange={(v) => mutate((tt) => {
              ruleSetter(tt, 'IPv4', 'domain', v);
              syncOutbound(tt, 'IPv4', ipv4Settings);
            })}
          />
        }
      />
    </>
  );
}
