import { useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Tag, Tooltip, message } from 'antd';
import {
  ApiOutlined,
  ApartmentOutlined,
  CheckCircleFilled,
  ClockCircleOutlined,
  CloseCircleFilled,
  CloudServerOutlined,
  ClusterOutlined,
  CodeOutlined,
  CopyOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  ExportOutlined,
  FileTextOutlined,
  GlobalOutlined,
  ImportOutlined,
  LoadingOutlined,
  MailOutlined,
  MessageOutlined,
  MoonOutlined,
  PlusOutlined,
  ReloadOutlined,
  SafetyOutlined,
  SearchOutlined,
  SettingOutlined,
  SunOutlined,
  SwapOutlined,
  TagsOutlined,
  TeamOutlined,
  ToolOutlined,
} from '@ant-design/icons';

import { ClipboardManager, HttpUtil, SizeFormatter } from '@/utils';
import { useInboundOptions } from '@/api/queries/useInboundOptions';
import { useAllSettings } from '@/api/queries/useAllSettings';
import { useTheme } from '@/hooks/useTheme';
import type { ClientRecord, InboundOption } from '@/schemas/client';
import { useCommandPalette } from './useCommandPalette';
import './CommandPalette.css';

interface PaletteItem {
  id: string;
  category: 'clients' | 'inbounds' | 'navigation' | 'settings' | 'actions';
  title: string;
  subtitle?: string;
  keywords?: string[];
  icon: ReactNode;
  tag?: ReactNode;
  action: () => void | Promise<void>;
  secondaryAction?: {
    label: string;
    icon: ReactNode;
    execute: (e: React.MouseEvent) => void;
  };
}

export default function CommandPalette() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { isDark, isUltra, toggleTheme, toggleUltra } = useTheme();
  const { isOpen, close } = useCommandPalette();
  const { allSetting } = useAllSettings();
  const { data: inbounds = [] } = useInboundOptions();

  const [query, setQuery] = useState('');
  const debouncedQuery = useDeferredValue(query.trim());
  const [clients, setClients] = useState<ClientRecord[]>([]);
  const [loadingClients, setLoadingClients] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);

  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (isOpen) {
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen || debouncedQuery.length < 1) {
      return;
    }

    let isCurrent = true;

    HttpUtil.get(
      `/panel/api/clients/list/paged?search=${encodeURIComponent(debouncedQuery)}&pageSize=8`,
      undefined,
      { silent: true },
    )
      .then((msg) => {
        if (!isCurrent) return;
        if (
          msg?.success &&
          msg?.obj &&
          Array.isArray((msg.obj as { items?: ClientRecord[] }).items)
        ) {
          setClients((msg.obj as { items: ClientRecord[] }).items);
        } else {
          setClients([]);
        }
      })
      .catch(() => {
        if (isCurrent) setClients([]);
      })
      .finally(() => {
        if (isCurrent) setLoadingClients(false);
      });

    return () => {
      isCurrent = false;
    };
  }, [isOpen, debouncedQuery]);

  const copySubscription = useCallback(
    async (client: ClientRecord) => {
      if (!client.subId || !allSetting.subURI) {
        message.warning(t('pages.clients.noSubId'));
        return;
      }
      const link = `${allSetting.subURI}${client.subId}`;
      const ok = await ClipboardManager.copyText(link);
      if (ok) message.success(t('copied'));
    },
    [allSetting.subURI, t],
  );

  const restartXray = useCallback(async () => {
    close();
    const msg = await HttpUtil.post('/panel/api/server/restartXrayService');
    if (msg?.success) {
      message.success(t('pages.index.toasts.restartedXray') || 'Xray restarted');
    }
  }, [close, t]);

  const cycleTheme = useCallback(() => {
    if (!isDark) {
      toggleTheme();
      if (isUltra) toggleUltra();
    } else if (!isUltra) {
      toggleUltra();
    } else {
      toggleUltra();
      toggleTheme();
    }
    close();
  }, [isDark, isUltra, toggleTheme, toggleUltra, close]);

  const isClientSearching = isOpen && debouncedQuery.length >= 1 && loadingClients;

  const items = useMemo<PaletteItem[]>(() => {
    const list: PaletteItem[] = [];
    const q = debouncedQuery.toLowerCase();

    const matches = (title: string, subtitle?: string, keywords: string[] = []) => {
      if (!q) return true;
      if (title.toLowerCase().includes(q)) return true;
      if (subtitle && subtitle.toLowerCase().includes(q)) return true;
      return keywords.some((k) => k.toLowerCase().includes(q));
    };

    if (debouncedQuery.length > 0 && clients.length > 0) {
      clients.forEach((c) => {
        const up = Number(c.traffic?.up || 0);
        const down = Number(c.traffic?.down || 0);
        const total = Number(c.traffic?.total || (c.totalGB ? c.totalGB * 1024 * 1024 * 1024 : 0));
        const trafficUsed = SizeFormatter.sizeFormat(up + down);
        const trafficTotal = total > 0 ? SizeFormatter.sizeFormat(total) : '∞';
        const isOnline = c.enable !== false;

        list.push({
          id: `client-${c.id ?? c.email}`,
          category: 'clients',
          title: c.email,
          subtitle: `${trafficUsed} / ${trafficTotal}${c.comment ? ` · ${c.comment}` : ''}`,
          icon: isOnline ? (
            <CheckCircleFilled style={{ color: '#52c41a' }} />
          ) : (
            <CloseCircleFilled style={{ color: '#ff4d4f' }} />
          ),
          action: () => {
            close();
            navigate(`/clients?search=${encodeURIComponent(c.email)}`);
          },
          secondaryAction:
            c.subId && allSetting.subURI
              ? {
                  label: t('commandPalette.copySubscription') || 'Copy subscription',
                  icon: <CopyOutlined />,
                  execute: (e) => {
                    e.stopPropagation();
                    copySubscription(c);
                  },
                }
              : undefined,
        });
      });
    }

    const matchedInbounds = inbounds.filter((ib: InboundOption) => {
      if (!q) return false;
      return (
        (ib.tag && ib.tag.toLowerCase().includes(q)) ||
        (ib.remark && ib.remark.toLowerCase().includes(q)) ||
        (ib.protocol && ib.protocol.toLowerCase().includes(q)) ||
        (ib.port && String(ib.port).includes(q))
      );
    });

    matchedInbounds.slice(0, 8).forEach((ib) => {
      list.push({
        id: `inbound-${ib.id}`,
        category: 'inbounds',
        title: ib.remark || ib.tag || `Inbound #${ib.id}`,
        subtitle: `Port ${ib.port || ''} · ${ib.protocol || ''}`,
        icon: <ImportOutlined style={{ color: '#1677ff' }} />,
        tag: ib.protocol ? <Tag color="blue">{ib.protocol.toUpperCase()}</Tag> : undefined,
        action: () => {
          close();
          navigate(
            `/inbounds?search=${encodeURIComponent(ib.tag || ib.remark || String(ib.port || ''))}`,
          );
        },
      });
    });

    const pages = [
      {
        path: '/',
        title: t('menu.dashboard'),
        subtitle: 'Overview & system telemetry',
        keywords: [
          'overview',
          'dashboard',
          'داشبورد',
          'نمای کلی',
          'cpu',
          'ram',
          'memory',
          'traffic',
          'speed',
        ],
        icon: <DashboardOutlined />,
      },
      {
        path: '/inbounds',
        title: t('menu.inbounds'),
        subtitle: 'Manage protocols & ports',
        keywords: [
          'inbounds',
          'ورودی',
          'پورت',
          'اینباند',
          'vless',
          'vmess',
          'reality',
          'trojan',
          'shadowsocks',
          'wireguard',
          'hysteria',
        ],
        icon: <ImportOutlined />,
      },
      {
        path: '/clients',
        title: t('menu.clients'),
        subtitle: 'Manage client accounts & limits',
        keywords: ['clients', 'مشتریان', 'کاربران', 'کلاینت', 'users', 'sub', 'traffic'],
        icon: <TeamOutlined />,
      },
      {
        path: '/groups',
        title: t('menu.groups'),
        subtitle: 'Client grouping & batch assignments',
        keywords: ['groups', 'گروه‌ها', 'دسته بندی', 'تگ'],
        icon: <TagsOutlined />,
      },
      {
        path: '/nodes',
        title: t('menu.nodes'),
        subtitle: 'Multi-node cluster management',
        keywords: ['nodes', 'نودها', 'سرورها', 'سرور', 'cluster', 'remote nodes'],
        icon: <ClusterOutlined />,
      },
      {
        path: '/hosts',
        title: t('menu.hosts'),
        subtitle: 'Custom SNI & domain routing hosts',
        keywords: ['hosts', 'هاست‌ها', 'دامنه', 'sni', 'domains'],
        icon: <GlobalOutlined />,
      },
      {
        path: '/outbound',
        title: t('menu.outbounds'),
        subtitle: 'Exit proxies & warp gateways',
        keywords: [
          'outbounds',
          'خروجی',
          'آتباند',
          'freedom',
          'blackhole',
          'socks',
          'http',
          'warp',
          'nord',
          'pia',
        ],
        icon: <ExportOutlined />,
      },
      {
        path: '/routing',
        title: t('menu.routing'),
        subtitle: 'Traffic routing & rule balancer',
        keywords: [
          'routing',
          'روتینگ',
          'مسیریابی',
          'قوانین',
          'rules',
          'geoip',
          'geosite',
          'direct',
          'block',
        ],
        icon: <SwapOutlined />,
      },
      {
        path: '/settings',
        title: t('menu.settings'),
        subtitle: 'Panel configuration & security',
        keywords: ['settings', 'تنظیمات', 'پنل', 'port', 'password', 'ssl', 'telegram'],
        icon: <SettingOutlined />,
      },
      {
        path: '/xray',
        title: t('menu.xray'),
        subtitle: 'Xray core templates & balancer configs',
        keywords: ['xray', 'کانفیگ', 'تنظیمات xray', 'template', 'balancer', 'dns'],
        icon: <ToolOutlined />,
      },
      {
        path: '/api-docs',
        title: t('menu.apiDocs'),
        subtitle: 'OpenAPI documentation & Swagger explorer',
        keywords: ['api', 'api docs', 'مستندات', 'swagger', 'rest api', 'endpoints'],
        icon: <ApiOutlined />,
      },
    ];

    pages
      .filter((p) => matches(p.title, p.subtitle, p.keywords))
      .forEach((p) => {
        list.push({
          id: `nav-${p.path}`,
          category: 'navigation',
          title: p.title,
          subtitle: p.subtitle,
          keywords: p.keywords,
          icon: p.icon,
          action: () => {
            close();
            navigate(p.path);
          },
        });
      });

    const settingsSubSections = [
      {
        path: '/settings#general',
        title: `${t('menu.settings')} · ${t('pages.settings.panelSettings')}`,
        subtitle: 'Web port, base path, listen IP, SSL certificates',
        keywords: [
          'general',
          'عمومی',
          'پورت پنل',
          'آدرس پنل',
          'ssl',
          'certificate',
          'webPort',
          'webBasePath',
          'listenIP',
          'گواهی',
        ],
        icon: <SettingOutlined />,
      },
      {
        path: '/settings#security',
        title: `${t('menu.settings')} · ${t('pages.settings.securitySettings')}`,
        subtitle: 'Admin credentials, password, Two-Factor 2FA, session limits',
        keywords: [
          'security',
          'امنیت',
          'رمز عبور',
          'نام کاربری',
          'دو مرحله‌ای',
          '2fa',
          'two factor',
          'password',
          'username',
          'login limit',
        ],
        icon: <SafetyOutlined />,
      },
      {
        path: '/settings#telegram',
        title: `${t('menu.settings')} · ${t('pages.settings.TGBotSettings')}`,
        subtitle: 'Telegram bot alerts, token, chat ID, backup scheduler',
        keywords: [
          'telegram',
          'تلگرام',
          'ربات تلگرام',
          'توکن',
          'tgbot',
          'bot token',
          'chat id',
          'notifications',
          'هشدار',
        ],
        icon: <MessageOutlined />,
      },
      {
        path: '/settings#email',
        title: `${t('menu.settings')} · ${t('pages.settings.emailSettings')}`,
        subtitle: 'SMTP mail service, server status and crash alerts',
        keywords: ['email', 'ایمیل', 'هشدار ایمیل', 'smtp', 'mail', 'crash alerts'],
        icon: <MailOutlined />,
      },
      {
        path: '/settings#subscription',
        title: `${t('menu.settings')} · ${t('pages.settings.subSettings')}`,
        subtitle: 'Subscription service port, domain, reverse proxy URI',
        keywords: [
          'subscription',
          'سابسکریپشن',
          'لینک اشتراک',
          'پورت ساب',
          'subPort',
          'subURI',
          'subDomain',
          'reverse proxy',
        ],
        icon: <CloudServerOutlined />,
      },
      {
        path: '/settings#subscription-formats',
        title: `${t('menu.settings')} · ${t('menu.subFormats')}`,
        subtitle: 'Clash, Sing-box, V2Ray JSON subscription endpoints',
        keywords: ['formats', 'فرمت ساب', 'clash', 'sing-box', 'v2ray', 'json', 'sub formats'],
        icon: <CodeOutlined />,
      },
      {
        path: '/settings#subscription-balancers',
        title: `${t('menu.settings')} · ${t('pages.settings.subBalancers.menu')}`,
        subtitle: 'Automated subscription node balancing',
        keywords: ['balancers', 'لود بالانسر ساب', 'sub balancers', 'balancer nodes'],
        icon: <ApartmentOutlined />,
      },
      {
        path: '/xray#basic',
        title: `${t('menu.xray')} · ${t('pages.xray.basicTemplate')}`,
        subtitle: 'Freedom strategies, happy eyeballs, connection & log policies',
        keywords: [
          'xray basics',
          'پایه',
          'عمومی',
          'freedom strategy',
          'happy eyeballs',
          'torrent',
          'connection',
        ],
        icon: <ToolOutlined />,
      },
      {
        path: '/xray#basic',
        title: `${t('menu.xray')} · ${t('pages.xray.metricsListen') || 'Metrics Endpoint'}`,
        subtitle: 'Prometheus metrics endpoint, inbound/outbound uplink & downlink stats',
        keywords: [
          'metrics',
          'متریک',
          'آمار',
          'prometheus',
          'statistics',
          'listen',
          'statsInbound',
          'statsOutbound',
          'metrics_out',
        ],
        icon: <DashboardOutlined />,
      },
      {
        path: '/xray#basic',
        title: `${t('menu.xray')} · ${t('pages.xray.connectionLimits') || 'Connection Limits'}`,
        subtitle: 'Idle timeout (connIdle) and handshake buffer size',
        keywords: [
          'limits',
          'محدودیت اتصال',
          'بافر',
          'idle timeout',
          'bufferSize',
          'connIdle',
          'timeout',
        ],
        icon: <ClockCircleOutlined />,
      },
      {
        path: '/xray#basic',
        title: `${t('menu.xray')} · ${t('pages.xray.logConfigs') || 'Log Configs'}`,
        subtitle: 'Log level (debug/info/warning/error), access log, error log, DNS log',
        keywords: [
          'logs',
          'لاگ xray',
          'سطح لاگ',
          'access log',
          'error log',
          'dns log',
          'mask address',
          'loglevel',
        ],
        icon: <FileTextOutlined />,
      },
      {
        path: '/xray#balancer',
        title: `${t('menu.xray')} · ${t('pages.xray.Balancers')}`,
        subtitle: 'Traffic load balancers, leastPing, roundRobin, fallback strategies',
        keywords: [
          'balancers',
          'بالانسر',
          'لود بالانسر',
          'balancer',
          'fallback',
          'leastPing',
          'roundRobin',
          'strategy',
        ],
        icon: <ClusterOutlined />,
      },
      {
        path: '/xray#dns',
        title: `${t('menu.xray')} · DNS Configs`,
        subtitle: 'Custom DNS upstream servers, hosts map, DoH, DoT, DoU',
        keywords: ['dns', 'دی ان اس', 'dns servers', 'hosts', 'doh', 'dot', 'cloudflare dns'],
        icon: <DatabaseOutlined />,
      },
      {
        path: '/xray#outbound',
        title: `${t('menu.xray')} · ${t('pages.xray.Outbounds')}`,
        subtitle: 'Freedom direct, blackhole, shadowsocks, socks outbounds',
        keywords: ['outbound', 'خروجی', 'آتباند', 'freedom', 'direct', 'proxy outbounds'],
        icon: <ExportOutlined />,
      },
      {
        path: '/xray#routing',
        title: `${t('menu.xray')} · ${t('pages.xray.basicRouting') || 'Routing Rules'}`,
        subtitle: 'Traffic routing rules, geoip/geosite domain & IP rules',
        keywords: [
          'routing',
          'قوانین مسیریابی',
          'routing rules',
          'geoip',
          'geosite',
          'block',
          'direct',
        ],
        icon: <SwapOutlined />,
      },
      {
        path: '/xray#advanced',
        title: `${t('menu.xray')} · ${t('pages.xray.advancedTemplate')}`,
        subtitle: 'Raw JSON configuration template for Xray Core',
        keywords: ['advanced', 'قالب پیشرفته', 'json template', 'advanced config', 'custom json'],
        icon: <CodeOutlined />,
      },
    ];

    settingsSubSections
      .filter((s) => matches(s.title, s.subtitle, s.keywords))
      .forEach((s) => {
        list.push({
          id: `setting-${s.path}-${s.title}`,
          category: 'settings',
          title: s.title,
          subtitle: s.subtitle,
          keywords: s.keywords,
          icon: s.icon,
          action: () => {
            close();
            navigate(s.path);
          },
        });
      });

    const actions: PaletteItem[] = [
      {
        id: 'act-restart-xray',
        category: 'actions',
        title: t('pages.index.toasts.restartedXray') || 'Restart Xray Service',
        subtitle: 'Hot-restart Xray Core backend',
        keywords: ['restart', 'ری‌استارت', 'راه‌اندازی مجدد', 'xray restart', 'reboot xray'],
        icon: <ReloadOutlined style={{ color: '#faad14' }} />,
        action: restartXray,
      },
      {
        id: 'act-cycle-theme',
        category: 'actions',
        title: 'Cycle Theme (Light → Dark → Ultra)',
        subtitle: `Currently: ${isUltra ? 'Ultra Dark' : isDark ? 'Dark' : 'Light'}`,
        keywords: ['theme', 'تم', 'تغییر تم', 'light', 'dark', 'ultra', 'روشن', 'تاریک'],
        icon: isDark ? <SunOutlined /> : <MoonOutlined />,
        action: cycleTheme,
      },
      {
        id: 'act-toggle-theme',
        category: 'actions',
        title: isDark ? 'Switch to Light Theme' : 'Switch to Dark Theme',
        subtitle: isDark ? 'Light mode (white)' : 'Dark mode',
        keywords: ['light', 'dark', 'روشن', 'تاریک', 'تم'],
        icon: isDark ? (
          <SunOutlined style={{ color: '#faad14' }} />
        ) : (
          <MoonOutlined style={{ color: '#1677ff' }} />
        ),
        action: () => {
          toggleTheme();
          close();
        },
      },
      {
        id: 'act-add-inbound',
        category: 'actions',
        title: 'New Inbound',
        subtitle: 'Create a new proxy port/protocol',
        keywords: ['add inbound', 'اینباند جدید', 'پورت جدید', 'create inbound', 'new port'],
        icon: <PlusOutlined style={{ color: '#52c41a' }} />,
        action: () => {
          close();
          navigate('/inbounds');
        },
      },
      {
        id: 'act-add-client',
        category: 'actions',
        title: 'New Client',
        subtitle: 'Create a new user account',
        keywords: ['add client', 'کلاینت جدید', 'کاربر جدید', 'create user', 'new client'],
        icon: <PlusOutlined style={{ color: '#52c41a' }} />,
        action: () => {
          close();
          navigate('/clients');
        },
      },
    ];

    actions.filter((a) => matches(a.title, a.subtitle, a.keywords)).forEach((a) => list.push(a));

    return list;
  }, [
    debouncedQuery,
    clients,
    inbounds,
    isDark,
    isUltra,
    allSetting.subURI,
    t,
    close,
    navigate,
    copySubscription,
    restartXray,
    cycleTheme,
    toggleTheme,
  ]);

  const clampedActiveIndex = Math.min(activeIndex, Math.max(0, items.length - 1));

  useEffect(() => {
    if (!listRef.current) return;
    const activeEl = listRef.current.querySelector(
      `.command-palette-item[data-index="${clampedActiveIndex}"]`,
    ) as HTMLElement | null;
    if (activeEl) {
      activeEl.scrollIntoView({ block: 'nearest' });
    }
  }, [clampedActiveIndex]);

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActiveIndex((prev) => (items.length ? (prev + 1) % items.length : 0));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActiveIndex((prev) => (items.length ? (prev - 1 + items.length) % items.length : 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const current = items[clampedActiveIndex];
      if (current) current.action();
    }
  };

  if (!isOpen) return null;

  let lastCategory = '';
  const themeModeClass = isUltra ? 'ultra' : isDark ? 'dark' : 'light';

  return (
    <div
      className={`command-palette-backdrop ${themeModeClass}`}
      role="presentation"
      onClick={(e) => {
        if (e.target === e.currentTarget) close();
      }}
    >
      <div
        className={`command-palette-modal ${themeModeClass}`}
        role="dialog"
        aria-modal="true"
        aria-label={t('commandPalette.title') || 'Command Palette'}
      >
        <div className="command-palette-header">
          {isClientSearching ? (
            <LoadingOutlined className="command-palette-search-icon spinning" />
          ) : (
            <SearchOutlined className="command-palette-search-icon" />
          )}
          <input
            ref={inputRef}
            className="command-palette-input"
            type="text"
            placeholder={
              t('commandPalette.placeholder') ||
              'Type a command or search clients, inbounds, pages...'
            }
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setLoadingClients(e.target.value.trim().length > 0);
            }}
            onKeyDown={handleKeyDown}
          />
        </div>

        <div className="command-palette-body" ref={listRef}>
          {!isClientSearching && items.length === 0 && (
            <div className="command-palette-empty">{t('noData')}</div>
          )}

          {items.map((item, index) => {
            const isFirstOfCategory = item.category !== lastCategory;
            lastCategory = item.category;

            const categoryLabel =
              item.category === 'clients'
                ? t('menu.clients')
                : item.category === 'inbounds'
                  ? t('menu.inbounds')
                  : item.category === 'navigation'
                    ? t('commandPalette.navigation') || 'Navigation'
                    : item.category === 'settings'
                      ? t('menu.settings') || 'Settings & Sections'
                      : t('commandPalette.actions') || 'Actions';

            return (
              <div key={item.id} className="command-palette-group">
                {isFirstOfCategory && (
                  <div className="command-palette-group-title">{categoryLabel}</div>
                )}
                <button
                  type="button"
                  className={`command-palette-item ${index === clampedActiveIndex ? 'active' : ''}`}
                  data-index={index}
                  onClick={() => item.action()}
                  onMouseEnter={() => setActiveIndex(index)}
                >
                  <div className="command-palette-item-main">
                    <span className="command-palette-item-icon">{item.icon}</span>
                    <div className="command-palette-item-content">
                      <span className="command-palette-item-title">{item.title}</span>
                      {item.subtitle && (
                        <span className="command-palette-item-subtitle">{item.subtitle}</span>
                      )}
                    </div>
                  </div>

                  <div className="command-palette-item-actions">
                    {item.tag}
                    {item.secondaryAction && (
                      <Tooltip
                        title={item.secondaryAction.label}
                        placement="top"
                        zIndex={2500}
                        rootClassName="command-palette-tooltip"
                      >
                        <button
                          type="button"
                          className="command-palette-action-btn"
                          onClick={item.secondaryAction.execute}
                          aria-label={item.secondaryAction.label}
                        >
                          {item.secondaryAction.icon}
                        </button>
                      </Tooltip>
                    )}
                  </div>
                </button>
              </div>
            );
          })}
        </div>

        <div className="command-palette-footer">
          <div className="command-palette-kbd-group">
            <span>
              <kbd className="command-palette-kbd">↑</kbd>
              <kbd className="command-palette-kbd">↓</kbd>
              {t('commandPalette.navigate') || 'Navigate'}
            </span>
            <span>
              <kbd className="command-palette-kbd">↵</kbd>
              {t('commandPalette.select') || 'Select'}
            </span>
            <span>
              <kbd className="command-palette-kbd">Esc</kbd>
              {t('close')}
            </span>
          </div>
          <span>3x-ui Command Palette</span>
        </div>
      </div>
    </div>
  );
}
