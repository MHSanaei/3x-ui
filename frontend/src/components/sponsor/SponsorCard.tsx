import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { CloseOutlined, ExportOutlined } from '@ant-design/icons';

import { withBasePath } from '@/api/http-init';
import type { Sponsor } from '@/generated/types';
import { pickLocale } from '@/lib/sponsors';
import './SponsorCard.css';

export type SponsorCardVariant = 'banner' | 'compact' | 'card';

export interface SponsorCardProps {
  sponsor: Sponsor;
  variant?: SponsorCardVariant;
  iconOnly?: boolean;
  onClose?: () => void;
}

function SponsorLogo({ sponsor }: { sponsor: Sponsor }) {
  const [failedSrc, setFailedSrc] = useState('');
  if (sponsor.logo && failedSrc !== sponsor.logo) {
    return (
      <img
        className="sponsor-logo"
        src={withBasePath(sponsor.logo)}
        alt=""
        loading="lazy"
        onError={() => setFailedSrc(sponsor.logo ?? '')}
      />
    );
  }
  return (
    <span className="sponsor-logo sponsor-logo-fallback" aria-hidden="true">
      {sponsor.name.slice(0, 1).toUpperCase()}
    </span>
  );
}

export default function SponsorCard({
  sponsor,
  variant = 'banner',
  iconOnly = false,
  onClose,
}: SponsorCardProps) {
  const { t, i18n } = useTranslation();
  const lang = i18n.resolvedLanguage || i18n.language || 'en';
  const title = pickLocale(sponsor.title, lang) || sponsor.name;
  const text = pickLocale(sponsor.text, lang);
  const tag = t('pages.sponsors.tag');

  if (iconOnly) {
    return (
      <a
        className="sponsor-card sponsor-card-icon"
        href={sponsor.link}
        target="_blank"
        rel="noopener noreferrer sponsored"
        title={`${tag} · ${title}`}
        aria-label={`${tag}: ${title}`}
      >
        <SponsorLogo sponsor={sponsor} />
      </a>
    );
  }

  return (
    <div className={`sponsor-card sponsor-card-${variant}`}>
      <a
        className="sponsor-main"
        href={sponsor.link}
        target="_blank"
        rel="noopener noreferrer sponsored"
      >
        <SponsorLogo sponsor={sponsor} />
        <span className="sponsor-body" dir="auto">
          <span className="sponsor-head">
            <span className="sponsor-tag">{tag}</span>
            <span className="sponsor-title">{title}</span>
          </span>
          {text && <span className="sponsor-text">{text}</span>}
        </span>
        {variant !== 'compact' && (
          <span className="sponsor-visit">
            {t('pages.sponsors.visit')} <ExportOutlined />
          </span>
        )}
      </a>
      {onClose && (
        <button
          type="button"
          className="sponsor-close"
          aria-label={t('close')}
          title={t('close')}
          onClick={onClose}
        >
          <CloseOutlined />
        </button>
      )}
    </div>
  );
}
