import { useEffect, useMemo, useRef } from 'react';
import type { CSSProperties } from 'react';
import { Trans, useTranslation } from 'react-i18next';
import { Card, Col, ConfigProvider, Layout, Row, Tabs, Typography } from 'antd';
import SwaggerUI from 'swagger-ui-react';
import 'swagger-ui-react/swagger-ui.css';

import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/layouts/AppSidebar';
import { EXAMPLES } from '@/generated/examples';
import { buildWebSocketEvents } from './websocket-events';
import { apiReferencePlugin, translateApiText } from './localization';
import { bindSwaggerLabels } from './swagger-labels';
import './ApiDocsPage.css';

const basePath = window.X_UI_BASE_PATH || '';
const openApiUrl = `${basePath}panel/api/openapi.json`;
const websocketEvents = buildWebSocketEvents(EXAMPLES);

interface TaggedOperations {
  keySeq: () => { first: () => string | undefined };
  filter: (keep: (operations: unknown, tag: string) => boolean) => TaggedOperations;
}

interface LayoutSelectors {
  currentFilter: () => string | false;
}

interface SectionTabsProps {
  specSelectors: { tags: () => { toJS: () => { name: string }[] } };
  layoutSelectors: LayoutSelectors;
  layoutActions: { updateFilter: (tag: string) => void };
}

function SectionTabs({ specSelectors, layoutSelectors, layoutActions }: SectionTabsProps) {
  const tags = specSelectors
    .tags()
    .toJS()
    .map((tag) => tag.name);
  return (
    <div className="wrapper section-tabs">
      <Tabs
        size="small"
        activeKey={layoutSelectors.currentFilter() || tags[0]}
        onChange={layoutActions.updateFilter}
        items={tags.map((tag) => ({ key: tag, label: tag }))}
      />
    </div>
  );
}

// Shows one tag at a time, the first until a tab is picked. Swagger's own filter is a
// substring match ("Settings" would also show "Xray Settings") and no-op while unset.
const sectionTabsPlugin = {
  statePlugins: {
    spec: {
      wrapSelectors: {
        taggedOperations:
          (
            select: (...args: unknown[]) => TaggedOperations,
            system: { getSystem: () => { layoutSelectors: LayoutSelectors } },
          ) =>
          (...args: unknown[]) => {
            const operations = select(...args);
            const active =
              system.getSystem().layoutSelectors.currentFilter() || operations.keySeq().first();
            return operations.filter((_, tag) => tag === active);
          },
      },
    },
  },
  components: { FilterContainer: SectionTabs },
};

export default function ApiDocsPage() {
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { t, i18n } = useTranslation();
  const docsRoot = useRef<HTMLDivElement>(null);
  const plugins = useMemo(() => [sectionTabsPlugin, apiReferencePlugin(t)], [t]);
  const docsStyle = {
    '--swagger-required-label': JSON.stringify(t('apiDocs.labels.requiredSuffix')),
  } as CSSProperties;

  useEffect(() => {
    if (i18n.resolvedLanguage !== 'sk-SK' || !docsRoot.current) return;
    return bindSwaggerLabels(docsRoot.current, t);
  }, [i18n.resolvedLanguage, t]);

  const pageClass = useMemo(() => {
    const classes = ['api-docs-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout className={pageClass} data-docs-language={i18n.resolvedLanguage} style={docsStyle}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content className="content-area">
            <Tabs
              items={[
                {
                  key: 'panel-api',
                  label: t('apiDocs.labels.panelTab'),
                  children: (
                    <div
                      ref={docsRoot}
                      className="docs-wrapper"
                      role="region"
                      aria-label={t('menu.apiDocs')}
                    >
                      <SwaggerUI
                        url={openApiUrl}
                        docExpansion="list"
                        deepLinking={false}
                        plugins={plugins}
                        tryItOutEnabled
                        persistAuthorization
                      />
                    </div>
                  ),
                },
                {
                  key: 'websocket-events',
                  label: t('apiDocs.labels.eventsTab'),
                  children: (
                    <section className="websocket-events">
                      <Typography.Paragraph>
                        <Trans
                          i18nKey="apiDocs.labels.websocketIntro"
                          components={{
                            handshake: <Typography.Text code>GET /ws</Typography.Text>,
                            envelope: (
                              <Typography.Text code>{'{ type, payload, time }'}</Typography.Text>
                            ),
                          }}
                        />
                      </Typography.Paragraph>
                      <Row gutter={[12, 12]}>
                        {websocketEvents.map((event) => (
                          <Col key={event.type} xs={24} sm={12} xl={8}>
                            <Card
                              size="small"
                              title={<Typography.Text code>{event.type}</Typography.Text>}
                            >
                              <Typography.Paragraph>
                                {translateApiText(event.summary, t)}
                              </Typography.Paragraph>
                              <pre>{JSON.stringify(event.example, null, 2)}</pre>
                            </Card>
                          </Col>
                        ))}
                      </Row>
                    </section>
                  ),
                },
              ]}
            />
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
