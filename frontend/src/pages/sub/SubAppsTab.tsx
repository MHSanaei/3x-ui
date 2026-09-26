import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Segmented } from 'antd';
import { AndroidOutlined, AppleOutlined } from '@ant-design/icons';

import { APP_ICONS } from './appIcons';
import type { AppPlatform, SubApp } from './subPageModel';

interface SubAppsTabProps {
  apps: Record<AppPlatform, SubApp[]>;
  initialPlatform: AppPlatform;
  onOpen: (url: string) => void;
}

const PLATFORM_OPTIONS = [
  { value: 'android' as const, label: 'Android', icon: <AndroidOutlined /> },
  { value: 'ios' as const, label: 'iOS', icon: <AppleOutlined /> },
];

function AppIcon({ name }: { name: string }) {
  const icon = APP_ICONS[name];
  if (!icon) {
    return (
      <span className="sub-app-mark" aria-hidden="true">
        {name.charAt(0)}
      </span>
    );
  }
  if (icon.tinted) {
    const mask = `url("${icon.src}")`;
    return (
      <span className="sub-app-mark" aria-hidden="true">
        <span className="sub-app-glyph" style={{ maskImage: mask, WebkitMaskImage: mask }} />
      </span>
    );
  }
  return <img className="sub-app-logo" src={icon.src} alt="" width={32} height={32} />;
}

export default function SubAppsTab({ apps, initialPlatform, onOpen }: SubAppsTabProps) {
  const { t } = useTranslation();
  const [platform, setPlatform] = useState<AppPlatform>(initialPlatform);

  return (
    <div className="sub-apps">
      <Segmented<AppPlatform> value={platform} onChange={setPlatform} options={PLATFORM_OPTIONS} />
      <div className="sub-app-grid">
        {apps[platform].map((app) => (
          <div key={app.name} className="sub-row">
            <AppIcon name={app.name} />
            <span className="sub-app-name">{app.name}</span>
            <Button type="primary" size="small" onClick={() => onOpen(app.url)}>
              {t('add')}
            </Button>
          </div>
        ))}
      </div>
    </div>
  );
}
