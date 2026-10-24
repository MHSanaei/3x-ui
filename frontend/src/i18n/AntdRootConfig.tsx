import type { ReactNode } from 'react';
import { QuestionCircleOutlined } from '@ant-design/icons';
import { ConfigProvider } from 'antd';
import type { ConfigProviderProps } from 'antd/es/config-provider';
import type { Locale } from 'antd/es/locale';

const antdLocaleLoaders: Record<string, () => Promise<{ default: Locale }>> = {
  'ar-EG': () => import('antd/es/locale/ar_EG'),
  'es-ES': () => import('antd/es/locale/es_ES'),
  'fa-IR': () => import('antd/es/locale/fa_IR'),
  'id-ID': () => import('antd/es/locale/id_ID'),
  'ja-JP': () => import('antd/es/locale/ja_JP'),
  'pt-BR': () => import('antd/es/locale/pt_BR'),
  'ru-RU': () => import('antd/es/locale/ru_RU'),
  'sk-SK': () => import('antd/es/locale/sk_SK'),
  'tr-TR': () => import('antd/es/locale/tr_TR'),
  'uk-UA': () => import('antd/es/locale/uk_UA'),
  'vi-VN': () => import('antd/es/locale/vi_VN'),
  'zh-CN': () => import('antd/es/locale/zh_CN'),
  'zh-TW': () => import('antd/es/locale/zh_TW'),
};

// Undefined keeps antd's built-in en_US.
let antdLocale: Locale | undefined;

export async function loadAntdLocale(language: string): Promise<void> {
  antdLocale = (await antdLocaleLoaders[language]?.())?.default;
}

// A Form.Item tooltip icon sits inside its <label>, so its "question-circle" name
// became part of every hinted field's name. FormField exposes the hint as a description.
const FORM_CONFIG: ConfigProviderProps['form'] = {
  tooltip: { icon: <QuestionCircleOutlined aria-hidden /> },
};

export function AntdRootConfig({ children }: { children: ReactNode }) {
  return (
    <ConfigProvider locale={antdLocale} form={FORM_CONFIG}>
      {children}
    </ConfigProvider>
  );
}
