import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, Col, ConfigProvider, Layout, Row, Typography } from 'antd';
import SwaggerUI from 'swagger-ui-react';
import 'swagger-ui-react/swagger-ui.css';

import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/layouts/AppSidebar';
import { EXAMPLES } from '@/generated/examples';
import { buildWebSocketEvents } from './websocket-events';
import './ApiDocsPage.css';

const basePath = window.X_UI_BASE_PATH || '';
const openApiUrl = `${basePath}panel/api/openapi.json`;
const websocketEvents = buildWebSocketEvents(EXAMPLES);

export default function ApiDocsPage() {
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { t } = useTranslation();

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
            <section className="websocket-events" aria-labelledby="websocket-events-title">
              <Typography.Title id="websocket-events-title" level={2}>
                WebSocket events
              </Typography.Title>
              <Typography.Paragraph>
                After the cookie-authenticated <Typography.Text code>GET /ws</Typography.Text>{' '}
                upgrade, every server message uses{' '}
                <Typography.Text code>{'{ type, payload, time }'}</Typography.Text>. The time value
                is Unix milliseconds.
              </Typography.Paragraph>
              <Row gutter={[12, 12]}>
                {websocketEvents.map((event) => (
                  <Col key={event.type} xs={24} sm={12} xl={8}>
                    <Card size="small" title={<Typography.Text code>{event.type}</Typography.Text>}>
                      <Typography.Paragraph>{event.summary}</Typography.Paragraph>
                      <pre>{JSON.stringify(event.example, null, 2)}</pre>
                    </Card>
                  </Col>
                ))}
              </Row>
            </section>
            <div className="docs-wrapper" role="region" aria-label={t('menu.apiDocs')}>
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
