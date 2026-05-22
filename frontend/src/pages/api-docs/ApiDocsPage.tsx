import { useCallback, useEffect, useMemo, useState } from 'react';
import type { ComponentType, MouseEvent } from 'react';
import { Button, Card, ConfigProvider, Input, Layout, Space } from 'antd';
import {
  ApiOutlined,
  CloudServerOutlined,
  ClusterOutlined,
  CompressOutlined,
  ExpandOutlined,
  GlobalOutlined,
  KeyOutlined,
  LinkOutlined,
  NodeIndexOutlined,
  SafetyCertificateOutlined,
  SaveOutlined,
  SearchOutlined,
  SettingOutlined,
  WifiOutlined,
} from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/components/AppSidebar';
import { sections as allSections } from './endpoints.js';
import EndpointSection from './EndpointSection';
import type { Section } from './EndpointSection';
import CodeBlock from './CodeBlock';
import '@/styles/page-cards.css';
import './ApiDocsPage.css';

const sectionIcons: Record<string, ComponentType<{ className?: string }>> = {
  authentication: SafetyCertificateOutlined,
  inbounds: NodeIndexOutlined,
  server: CloudServerOutlined,
  nodes: ClusterOutlined,
  'custom-geo': GlobalOutlined,
  backup: SaveOutlined,
  settings: SettingOutlined,
  'api-tokens': KeyOutlined,
  'xray-settings': WifiOutlined,
  subscription: LinkOutlined,
  websocket: ApiOutlined,
};

const curlExample = `curl -X GET \\
  -H "Authorization: Bearer YOUR_API_TOKEN" \\
  -H "Accept: application/json" \\
  https://your-panel.example.com/panel/api/inbounds/list`;

const basePath = window.X_UI_BASE_PATH || '';
const requestUri = window.location.pathname;
const settingsHref = `${basePath}panel/settings#security`;

const endpointCount = (allSections as Section[]).reduce(
  (sum, s) => sum + s.endpoints.length,
  0,
);

export default function ApiDocsPage() {
  const { isDark, isUltra, antdThemeConfig } = useTheme();

  const [searchQuery, setSearchQuery] = useState('');
  const [collapsedSections, setCollapsedSections] = useState<Set<string>>(() => new Set());
  const [activeSection, setActiveSection] = useState('');

  const sections = useMemo<Section[]>(() => {
    const q = searchQuery.toLowerCase().trim();
    if (!q) return allSections as Section[];
    return (allSections as Section[])
      .map((s) => ({
        ...s,
        endpoints: s.endpoints.filter((e) =>
          e.path.toLowerCase().includes(q)
          || e.summary?.toLowerCase().includes(q)
          || e.method.toLowerCase().includes(q),
        ),
      }))
      .filter((s) => s.endpoints.length > 0);
  }, [searchQuery]);

  const visibleEndpoints = useMemo(
    () => sections.reduce((sum, s) => sum + s.endpoints.length, 0),
    [sections],
  );

  const toggleSection = useCallback((id: string) => {
    setCollapsedSections((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }, []);

  const expandAll = useCallback(() => setCollapsedSections(new Set()), []);
  const collapseAll = useCallback(
    () => setCollapsedSections(new Set((allSections as Section[]).map((s) => s.id))),
    [],
  );

  const scrollToSection = useCallback((id: string) => (e: MouseEvent) => {
    e.preventDefault();
    const el = document.getElementById(id);
    if (!el) return;
    el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    if (window.location.hash !== `#${id}`) {
      history.replaceState(null, '', `#${id}`);
    }
  }, []);

  useEffect(() => {
    const onHashChange = () => {
      const id = window.location.hash.slice(1);
      if (!id) return;
      const el = document.getElementById(id);
      if (el) el.scrollIntoView({ behavior: 'auto', block: 'start' });
    };
    requestAnimationFrame(onHashChange);
    window.addEventListener('hashchange', onHashChange);
    return () => window.removeEventListener('hashchange', onHashChange);
  }, []);

  useEffect(() => {
    const onScroll = () => {
      const toc = document.querySelector('.toc-nav');
      const tocHeight = toc instanceof HTMLElement ? toc.offsetHeight : 56;
      let current = '';
      for (const s of sections) {
        const el = document.getElementById(s.id);
        if (!el) continue;
        const rect = el.getBoundingClientRect();
        if (rect.top <= tocHeight + 20) {
          current = s.id;
        }
      }
      setActiveSection(current);
    };
    window.addEventListener('scroll', onScroll, { passive: true });
    requestAnimationFrame(onScroll);
    return () => window.removeEventListener('scroll', onScroll);
  }, [sections]);

  const pageClass = useMemo(() => {
    const classes = ['api-docs-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout className={pageClass}>
        <AppSidebar basePath={basePath} requestUri={requestUri} />

        <Layout className="content-shell">
          <Layout.Content className="content-area">
            <div className="docs-wrapper">
              <header className="docs-header">
                <h1 className="docs-title">API Documentation</h1>
                <p className="docs-lead">
                  The 3x-ui panel exposes a REST API under <code>/panel/api/</code>. Authenticate with the panel session
                  cookie, or with the <code>Authorization: Bearer &lt;token&gt;</code> header below. Every endpoint
                  returns a uniform <code>{'{ success, msg, obj }'}</code> envelope unless otherwise noted.
                </p>
              </header>

              <Card className="token-card" size="small">
                <div className="token-card-head">
                  <div className="token-card-title">
                    <KeyOutlined />
                    <span>API Tokens</span>
                  </div>
                  <Button type="primary" size="small" href={settingsHref}>
                    Manage tokens
                  </Button>
                </div>
                <p className="token-hint">
                  Create, enable, or revoke named Bearer tokens in{' '}
                  <a href={settingsHref}>Settings → Security</a>. Send each request as{' '}
                  <code>Authorization: Bearer &lt;token&gt;</code>. Token-authenticated callers skip CSRF and don&apos;t
                  need a session cookie. Deleting a token revokes it immediately — running bots will need a new one.
                </p>
              </Card>

              <Card className="curl-card" size="small" title="Quick example">
                <CodeBlock code={curlExample} lang="text" />
              </Card>

              <div className="toolbar">
                <Input
                  className="search-bar"
                  prefix={<SearchOutlined />}
                  placeholder="Search endpoints by path, method, or description…"
                  allowClear
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
                {searchQuery && (
                  <span className="match-count">
                    {visibleEndpoints} / {endpointCount} endpoints
                  </span>
                )}
                <Space size="small">
                  <Button size="small" icon={<ExpandOutlined />} onClick={expandAll}>
                    Expand all
                  </Button>
                  <Button size="small" icon={<CompressOutlined />} onClick={collapseAll}>
                    Collapse all
                  </Button>
                </Space>
              </div>

              <nav className="toc-nav">
                <span className="toc-label">On this page:</span>
                <div className="toc-links">
                  {sections.map((s) => {
                    const Icon = sectionIcons[s.id];
                    return (
                      <a
                        key={s.id}
                        className={`toc-link${activeSection === s.id ? ' active' : ''}`}
                        href={`#${s.id}`}
                        onClick={scrollToSection(s.id)}
                      >
                        {Icon && <Icon />}
                        <span className="toc-text">{s.title}</span>
                        <span className="toc-badge">{s.endpoints.length}</span>
                      </a>
                    );
                  })}
                </div>
              </nav>

              {sections.map((s) => (
                <EndpointSection
                  key={s.id}
                  section={s}
                  icon={sectionIcons[s.id]}
                  collapsed={collapsedSections.has(s.id)}
                  onToggle={() => toggleSection(s.id)}
                />
              ))}
            </div>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
