import { Tag } from 'antd';
import { useTranslation } from 'react-i18next';

import { useFactoryDefaults } from '@/api/queries/useFactoryDefaults';

/**
 * Value semantics on purpose: the tag answers "does this equal the shipped
 * default?", not "has the user ever saved this key?" — a stored 2096 and a
 * fallback 2096 behave identically, so they read identically.
 */
export function matchesFactoryDefault(current: unknown, factoryDefault: string | undefined): boolean {
  if (factoryDefault === undefined) return false;
  if (typeof current === 'number') {
    const parsed = Number(factoryDefault);
    return factoryDefault.trim() !== '' && !Number.isNaN(parsed) && parsed === current;
  }
  if (typeof current === 'boolean') {
    if (factoryDefault !== 'true' && factoryDefault !== 'false') return false;
    return (factoryDefault === 'true') === current;
  }
  if (typeof current === 'string') return factoryDefault === current;
  return false;
}

interface DefaultSettingTagProps {
  settingKey: string;
  value: unknown;
}

export default function DefaultSettingTag({ settingKey, value }: DefaultSettingTagProps) {
  const { t } = useTranslation();
  const defaults = useFactoryDefaults();

  if (!matchesFactoryDefault(value, defaults.data?.[settingKey])) return null;

  return <Tag style={{ marginLeft: 8 }}>{t('pages.settings.defaultTag')}</Tag>;
}
