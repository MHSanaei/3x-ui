import type { ReactNode } from 'react';
import { ConfigProvider } from 'antd';
import skSK from 'antd/locale/sk_SK';
import { useTranslation } from 'react-i18next';
import 'dayjs/locale/sk';

export function LocaleProvider({ children }: { children: ReactNode }) {
  const { i18n } = useTranslation();
  return (
    <ConfigProvider locale={i18n.resolvedLanguage === 'sk-SK' ? skSK : undefined}>
      {children}
    </ConfigProvider>
  );
}
