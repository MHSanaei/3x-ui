import { useEffect } from 'react';
import type { Decorator, Preview } from '@storybook/react-vite';
import { ConfigProvider } from 'antd';
import i18next from 'i18next';
import { initReactI18next } from 'react-i18next';

import { buildAntdThemeConfig } from '@/hooks/useTheme';
import enUS from '../../internal/web/translation/en-US.json';

if (!i18next.isInitialized) {
  void i18next.use(initReactI18next).init({
    lng: 'en-US',
    fallbackLng: 'en-US',
    resources: { 'en-US': { translation: enUS } },
    interpolation: { escapeValue: false, prefix: '{', suffix: '}' },
    returnNull: false,
  });
}

const withTheme: Decorator = (Story, context) => {
  const dark = context.globals.theme === 'dark';
  useEffect(() => {
    document.body.setAttribute('class', dark ? 'dark' : 'light');
    document.documentElement.setAttribute('data-theme', dark ? 'dark' : 'light');
  }, [dark]);
  return (
    <ConfigProvider theme={buildAntdThemeConfig(dark, false)}>
      <div style={{ padding: 24, minWidth: 320 }}>
        <Story />
      </div>
    </ConfigProvider>
  );
};

const preview: Preview = {
  decorators: [withTheme],
  globalTypes: {
    theme: {
      description: 'Ant Design theme',
      defaultValue: 'light',
      toolbar: {
        title: 'Theme',
        icon: 'circlehollow',
        items: [
          { value: 'light', title: 'Light' },
          { value: 'dark', title: 'Dark' },
        ],
        dynamicTitle: true,
      },
    },
  },
  parameters: {
    controls: {
      expanded: true,
      sort: 'requiredFirst',
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
    a11y: {
      test: 'error',
    },
  },
};

export default preview;
