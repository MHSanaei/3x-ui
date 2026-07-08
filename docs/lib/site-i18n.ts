import type { Locale } from './i18n';

// UI strings for the marketing chrome (landing page hero/features/footer + the
// shared navbar labels). The docs *pages* are translated as MDX under
// content/docs/{locale}; this covers the React-rendered home page and nav that
// can't live in MDX. English is the source; fa/ru/zh fall back to en.
//
// Convention matches the docs: translate prose only — product/protocol names
// (3x-ui, Xray, VLESS, REALITY, x25519, Docker, REST API, …) stay in Latin.
export interface SiteMessages {
  tagline: string;
  getStarted: string;
  viewOnGitHub: string;
  documentation: string;
  donate: string;
  docs: string;
  stars: string;
  forks: string;
  latest: string;
  copyCommand: string;
  copied: string;
  featuresHeading: string;
  featuresSubtitle: string;
  // Order matches the icon list in components/home/features.tsx.
  features: { title: string; description: string }[];
  // Footer license line: `{app} — {before}<a>GPL-3.0</a>{after}` (spacing baked in).
  licenseBefore: string;
  licenseAfter: string;
}

const en: SiteMessages = {
  tagline: 'Advanced web panel for managing Xray-core servers',
  getStarted: 'Get started',
  viewOnGitHub: 'View on GitHub',
  documentation: 'Documentation',
  donate: 'Donate',
  docs: 'Docs',
  stars: 'stars',
  forks: 'forks',
  latest: 'latest',
  copyCommand: 'Copy install command',
  copied: 'Copied',
  featuresHeading: 'Everything you need to run Xray',
  featuresSubtitle:
    'A modern, fast control panel for Xray-core — built for operators who want power without the command-line grind.',
  features: [
    {
      title: 'Every major protocol',
      description:
        'VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, SOCKS, HTTP and Dokodemo-door — managed from one panel.',
    },
    {
      title: 'REALITY & XTLS-Vision',
      description:
        'First-class support for VLESS + REALITY with x25519 keys, short IDs and the xtls-rprx-vision flow for stealth and speed.',
    },
    {
      title: 'Clients & traffic control',
      description:
        'Per-client traffic quotas, expiry dates, IP limits and live online status, with one-click share links and QR codes.',
    },
    {
      title: 'Multi-node & subscriptions',
      description:
        'Coordinate multiple servers, managed hosts and external proxies, and serve VLESS / Clash / JSON subscriptions.',
    },
    {
      title: 'Telegram bot & alerts',
      description:
        'Built-in Telegram notifications for traffic caps, expiry warnings and system load, plus admin actions.',
    },
    {
      title: 'Self-hosted & scriptable',
      description:
        'A single Go binary or Docker image, an SQLite/PostgreSQL backend, and a full REST API for automation.',
    },
  ],
  licenseBefore: 'released under the ',
  licenseAfter: ' license.',
};

const fa: SiteMessages = {
  tagline: 'پنل وب پیشرفته برای مدیریت سرورهای Xray-core',
  getStarted: 'شروع کنید',
  viewOnGitHub: 'مشاهده در GitHub',
  documentation: 'مستندات',
  donate: 'حمایت مالی',
  docs: 'مستندات',
  stars: 'ستاره',
  forks: 'فورک',
  latest: 'آخرین',
  copyCommand: 'کپی دستور نصب',
  copied: 'کپی شد',
  featuresHeading: 'هر آنچه برای اجرای Xray لازم دارید',
  featuresSubtitle:
    'یک پنل کنترلِ مدرن و سریع برای Xray-core — ساخته‌شده برای ادمین‌هایی که قدرت می‌خواهند، بدون درگیری با خط فرمان.',
  features: [
    {
      title: 'همه‌ی پروتکل‌های اصلی',
      description:
        'VLESS، VMess، Trojan، Shadowsocks، WireGuard، Hysteria2، SOCKS، HTTP و Dokodemo-door — همه از یک پنل مدیریت می‌شوند.',
    },
    {
      title: 'REALITY و XTLS-Vision',
      description:
        'پشتیبانی درجه‌یک از VLESS + REALITY با کلیدهای x25519، short ID‌ها و فلوی xtls-rprx-vision برای مخفی‌کاری و سرعت.',
    },
    {
      title: 'کلاینت‌ها و کنترل ترافیک',
      description:
        'سهمیه‌ی ترافیک برای هر کلاینت، تاریخ انقضا، محدودیت IP و وضعیت آنلاینِ زنده، همراه با لینک‌های اشتراک‌گذاری و کدهای QR تنها با یک کلیک.',
    },
    {
      title: 'چندنودی و سابسکریپشن‌ها',
      description:
        'هماهنگ‌سازی چند سرور، هاست‌های مدیریت‌شده و پروکسی‌های خارجی، و ارائه‌ی سابسکریپشن‌های VLESS / Clash / JSON.',
    },
    {
      title: 'ربات Telegram و هشدارها',
      description:
        'اعلان‌های داخلیِ Telegram برای سقف ترافیک، هشدار انقضا و بار سیستم، به‌علاوه‌ی کنش‌های مدیریتی.',
    },
    {
      title: 'خودمیزبان و قابل‌اسکریپت',
      description:
        'یک باینری Go یا ایمیج Docker، بک‌اندِ SQLite/PostgreSQL، و یک REST API کامل برای خودکارسازی.',
    },
  ],
  licenseBefore: 'تحت مجوز ',
  licenseAfter: ' منتشر شده است.',
};

const ru: SiteMessages = {
  tagline: 'Продвинутая веб-панель для управления серверами Xray-core',
  getStarted: 'Начать',
  viewOnGitHub: 'Открыть на GitHub',
  documentation: 'Документация',
  donate: 'Поддержать',
  docs: 'Документация',
  stars: 'звёзд',
  forks: 'форков',
  latest: 'последняя',
  copyCommand: 'Скопировать команду установки',
  copied: 'Скопировано',
  featuresHeading: 'Всё необходимое для запуска Xray',
  featuresSubtitle:
    'Современная и быстрая панель управления для Xray-core — создана для администраторов, которым нужна мощь без возни с командной строкой.',
  features: [
    {
      title: 'Все основные протоколы',
      description:
        'VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, SOCKS, HTTP и Dokodemo-door — под управлением из одной панели.',
    },
    {
      title: 'REALITY и XTLS-Vision',
      description:
        'Первоклассная поддержка VLESS + REALITY с ключами x25519, short ID и потоком xtls-rprx-vision для скрытности и скорости.',
    },
    {
      title: 'Клиенты и контроль трафика',
      description:
        'Квоты трафика по клиентам, даты окончания, лимиты IP и статус «онлайн» в реальном времени, плюс ссылки-подписки и QR-коды в один клик.',
    },
    {
      title: 'Мультинода и подписки',
      description:
        'Координация нескольких серверов, управляемых хостов и внешних прокси, а также выдача подписок VLESS / Clash / JSON.',
    },
    {
      title: 'Telegram-бот и оповещения',
      description:
        'Встроенные уведомления Telegram о лимитах трафика, истечении срока и нагрузке системы, а также действия администратора.',
    },
    {
      title: 'Свой хостинг и скрипты',
      description:
        'Один бинарный файл Go или Docker-образ, бэкенд SQLite/PostgreSQL и полноценный REST API для автоматизации.',
    },
  ],
  licenseBefore: 'распространяется под лицензией ',
  licenseAfter: '.',
};

const zh: SiteMessages = {
  tagline: '用于管理 Xray-core 服务器的高级 Web 面板',
  getStarted: '开始使用',
  viewOnGitHub: '在 GitHub 上查看',
  documentation: '文档',
  donate: '捐赠',
  docs: '文档',
  stars: '星标',
  forks: '复刻',
  latest: '最新',
  copyCommand: '复制安装命令',
  copied: '已复制',
  featuresHeading: '运行 Xray 所需的一切',
  featuresSubtitle:
    '为 Xray-core 打造的现代、快速控制面板 —— 专为想要强大功能又不愿折腾命令行的运维者而生。',
  features: [
    {
      title: '支持所有主流协议',
      description:
        'VLESS、VMess、Trojan、Shadowsocks、WireGuard、Hysteria2、SOCKS、HTTP 和 Dokodemo-door —— 全部在一个面板中管理。',
    },
    {
      title: 'REALITY 与 XTLS-Vision',
      description:
        '一流支持 VLESS + REALITY，配备 x25519 密钥、short ID 和 xtls-rprx-vision 流，兼顾隐蔽与速度。',
    },
    {
      title: '客户端与流量控制',
      description:
        '为每个客户端设置流量配额、到期日期、IP 限制和实时在线状态，并支持一键分享链接和二维码。',
    },
    {
      title: '多节点与订阅',
      description: '协调多台服务器、托管主机和外部代理，并提供 VLESS / Clash / JSON 订阅。',
    },
    {
      title: 'Telegram 机器人与告警',
      description: '内置 Telegram 通知，覆盖流量上限、到期提醒和系统负载，并支持管理员操作。',
    },
    {
      title: '自托管且可脚本化',
      description: '单个 Go 二进制文件或 Docker 镜像、SQLite/PostgreSQL 后端，以及用于自动化的完整 REST API。',
    },
  ],
  licenseBefore: '基于 ',
  licenseAfter: ' 许可证发布。',
};

const messages: Record<Locale, SiteMessages> = { en, fa, ru, zh };

export function getSiteMessages(lang: string): SiteMessages {
  return messages[lang as Locale] ?? en;
}
