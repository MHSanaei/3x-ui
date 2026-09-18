import { useId, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Input, Modal, Space, Tabs, Typography } from 'antd';

import JsonEditor from '@/components/form/JsonEditor';
import type { HappRoutingProfile } from '@/schemas/happRouting';
import {
  buildHappRoutingDeeplink,
  loadHappRouting,
  parseHappRoutingJson,
  parseHappRoutingList,
  type HappRoutingListKey,
} from './happRoutingEditor';

interface HappRoutingEditorModalProps {
  input: string;
  onCancel: () => void;
  onGenerate: (deeplink: string) => void;
}

const basicFields: { key: HappRoutingListKey; label: string; placeholder: string }[] = [
  {
    key: 'DirectSites',
    label: 'subHappDirectDomains',
    placeholder: 'domain:ir\ndomain:cn\nexample.local',
  },
  {
    key: 'ProxySites',
    label: 'subHappProxyDomains',
    placeholder: 'geosite:google\nyoutube.com',
  },
  {
    key: 'BlockSites',
    label: 'subHappBlockDomains',
    placeholder: 'geosite:category-ads-all\nanalytics.google.com',
  },
  {
    key: 'DirectIp',
    label: 'subHappDirectIPs',
    placeholder: 'geoip:ir\n192.168.0.0/16\n10.0.0.0/8',
  },
  {
    key: 'ProxyIp',
    label: 'subHappProxyIPs',
    placeholder: '1.1.1.1/32\n8.8.8.8/32',
  },
  {
    key: 'BlockIp',
    label: 'subHappBlockIPs',
    placeholder: 'geoip:phishing\n0.0.0.0/8',
  },
];

type BasicBuffers = Record<HappRoutingListKey, string>;

function basicBuffers(profile: HappRoutingProfile | null): BasicBuffers {
  return {
    DirectSites: profile?.DirectSites?.join('\n') ?? '',
    ProxySites: profile?.ProxySites?.join('\n') ?? '',
    BlockSites: profile?.BlockSites?.join('\n') ?? '',
    DirectIp: profile?.DirectIp?.join('\n') ?? '',
    ProxyIp: profile?.ProxyIp?.join('\n') ?? '',
    BlockIp: profile?.BlockIp?.join('\n') ?? '',
  };
}

function mergeBasicRules(profile: HappRoutingProfile, buffers: BasicBuffers): HappRoutingProfile {
  const next = { ...profile };
  // Only replace edited lists; absent lists and all other profile fields must survive unchanged.
  for (const { key } of basicFields) {
    if (buffers[key] !== (profile[key]?.join('\n') ?? '')) {
      next[key] = parseHappRoutingList(buffers[key]);
    }
  }
  return next;
}

const loadErrorKeys = {
  off: 'subHappEditorLoadOff',
  remote: 'subHappEditorLoadRemote',
  invalid: 'subHappEditorLoadInvalid',
};

export default function HappRoutingEditorModal({
  input,
  onCancel,
  onGenerate,
}: HappRoutingEditorModalProps) {
  const { t } = useTranslation();
  const fieldId = useId();
  const [loaded] = useState(() => loadHappRouting(input));
  const [profile, setProfile] = useState(() => (loaded.success ? loaded.profile : null));
  const [activeTab, setActiveTab] = useState('basic');
  // Keep raw buffers while typing so trailing newlines and temporarily invalid JSON are not lost.
  const [buffers, setBuffers] = useState(() => basicBuffers(profile));
  const [jsonText, setJsonText] = useState(() => (profile ? JSON.stringify(profile, null, 2) : ''));
  const advancedProfile = activeTab === 'advanced' ? parseHappRoutingJson(jsonText) : null;
  const invalidJson = activeTab === 'advanced' && advancedProfile === null;

  const switchTab = (nextTab: string) => {
    if (!profile || nextTab === activeTab) return;
    if (nextTab === 'advanced') {
      const next = mergeBasicRules(profile, buffers);
      setProfile(next);
      setJsonText(JSON.stringify(next, null, 2));
    } else {
      if (!advancedProfile) return;
      setProfile(advancedProfile);
      setBuffers(basicBuffers(advancedProfile));
    }
    setActiveTab(nextTab);
  };

  const generate = () => {
    if (!loaded.success || !profile) return;
    const next = activeTab === 'advanced' ? advancedProfile : mergeBasicRules(profile, buffers);
    if (next) onGenerate(buildHappRoutingDeeplink(next, loaded.mode));
  };

  return (
    <Modal
      title={t('pages.settings.subHappModalTitle')}
      open
      onCancel={onCancel}
      onOk={generate}
      okText={t('pages.settings.subHappBuildDeeplink')}
      okButtonProps={{ disabled: !loaded.success || invalidJson }}
      width={650}
    >
      <Space orientation="vertical" style={{ width: '100%', marginTop: 12 }} size="middle">
        {!loaded.success ? (
          <Alert type="error" showIcon title={t(`pages.settings.${loadErrorKeys[loaded.error]}`)} />
        ) : loaded.isNew ? (
          <Alert type="info" showIcon title={t('pages.settings.subHappEditorNew')} />
        ) : null}
        {loaded.success ? (
          <Tabs
            activeKey={activeTab}
            onChange={switchTab}
            destroyOnHidden
            items={[
              {
                key: 'basic',
                label: t('pages.settings.subHappEditorBasic'),
                disabled: invalidJson,
                children: (
                  <Space orientation="vertical" style={{ width: '100%' }} size="middle">
                    <Typography.Text type="secondary" id={`${fieldId}-hint`}>
                      {t('pages.settings.subHappEditorListHint')}
                    </Typography.Text>
                    {basicFields.map(({ key, label, placeholder }) => (
                      <div key={key}>
                        <label
                          htmlFor={`${fieldId}-${key}`}
                          style={{ display: 'block', fontWeight: 600, marginBottom: 4 }}
                        >
                          {t(`pages.settings.${label}`)}
                        </label>
                        <Input.TextArea
                          id={`${fieldId}-${key}`}
                          aria-describedby={`${fieldId}-hint`}
                          rows={2}
                          value={buffers[key]}
                          placeholder={placeholder}
                          onChange={(event) =>
                            setBuffers((previous) => ({ ...previous, [key]: event.target.value }))
                          }
                        />
                      </div>
                    ))}
                  </Space>
                ),
              },
              {
                key: 'advanced',
                label: t('pages.settings.subHappEditorAdvanced'),
                children: (
                  <Space orientation="vertical" style={{ width: '100%' }} size="middle">
                    <Typography.Text type="secondary">
                      {t('pages.settings.subHappEditorAdvancedHint')}
                    </Typography.Text>
                    {invalidJson ? (
                      <Alert
                        type="error"
                        showIcon
                        title={t('pages.settings.subHappEditorInvalidJson')}
                      />
                    ) : null}
                    <JsonEditor
                      value={jsonText}
                      onChange={setJsonText}
                      minHeight="320px"
                      maxHeight="50vh"
                    />
                  </Space>
                ),
              },
            ]}
          />
        ) : null}
      </Space>
    </Modal>
  );
}
