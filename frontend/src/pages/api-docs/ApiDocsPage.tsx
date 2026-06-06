import { useMemo } from 'react';
import { ConfigProvider, Layout } from 'antd';
import SwaggerUI from 'swagger-ui-react';
import 'swagger-ui-react/swagger-ui.css';

import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/layouts/AppSidebar';
import './ApiDocsPage.css';

const basePath = window.X_UI_BASE_PATH || '';
const openApiUrl = `${basePath}panel/api/openapi.json`;

export default function ApiDocsPage() {
  const { isDark, isUltra, antdThemeConfig } = useTheme();

  const pageClass = useMemo(() => {
    const classes = ['api-docs-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout className={pageClass}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content className="content-area">
            <div className="docs-wrapper">
              <SwaggerUI
                url={openApiUrl}
                docExpansion="list"
                deepLinking={false}
                tryItOutEnabled
                persistAuthorization
              />
            </div>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
