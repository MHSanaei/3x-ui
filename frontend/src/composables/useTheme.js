import { reactive, computed, watchEffect } from 'vue';
import { theme as antdTheme } from 'ant-design-vue';

// Single shared theme state. `import { theme } from '@/composables/useTheme.js'`
// from any component to read/toggle. Boot side-effects (apply current
// theme to <body>/<html>) run once at module load so the page is in the
// right theme before Vue mounts.

const STORAGE_DARK = 'dark-mode';
const STORAGE_ULTRA = 'isUltraDarkThemeEnabled';

function readBool(key, fallback) {
  const raw = localStorage.getItem(key);
  if (raw === null) return fallback;
  return raw === 'true';
}

const isDark = readBool(STORAGE_DARK, true);
const isUltra = readBool(STORAGE_ULTRA, false);

export const theme = reactive({
  isDark,
  isUltra,
});

export const currentTheme = computed(() => (theme.isDark ? 'dark' : 'light'));

// AD-Vue 4 theme config consumed by every page's <a-config-provider>.
// Three modes — light / dark / ultra-dark — all share AD-Vue's vanilla
// blue primary. Dark uses a navy palette across page/cards/modals so
// the sidebar blends with the rest of the surface; ultra-dark stays
// neutral black on top of darkAlgorithm.
const DARK_TOKENS = {
  colorBgBase: '#0a1426',
  colorBgLayout: '#0a1426',
  colorBgContainer: '#142340',
  colorBgElevated: '#1a2c4d',
};
const ULTRA_DARK_TOKENS = {
  colorBgBase: '#000',
  colorBgLayout: '#000',
  colorBgContainer: '#0a0a0a',
  colorBgElevated: '#141414',
};

// AD-Vue 4 hardcodes navy `#001529` / `#002140` as the Layout sider
// + trigger backgrounds and `#001529` / `#000c17` as the dark Menu item
// backgrounds (see node_modules/ant-design-vue/es/{layout,menu}/style/
// index.js). Override at the component-token level so the sider blends
// with darkAlgorithm's neutral surfaces.
// Dark theme uses a refined navy for the sidebar — distinct from the
// neutral ultra-dark and warmer than AD-Vue's stock #001529.
const DARK_LAYOUT_TOKENS = {
  colorBgHeader: '#0d1d33',
  colorBgTrigger: '#15294a',
  colorBgBody: '#000',
};
const ULTRA_DARK_LAYOUT_TOKENS = {
  colorBgHeader: '#0a0a0a',
  colorBgTrigger: '#141414',
  colorBgBody: '#000',
};
const DARK_MENU_TOKENS = {
  colorItemBg: '#0d1d33',
  colorSubItemBg: '#08142a',
  menuSubMenuBg: '#0d1d33',
};
const ULTRA_DARK_MENU_TOKENS = {
  colorItemBg: '#0a0a0a',
  colorSubItemBg: '#000',
  menuSubMenuBg: '#0a0a0a',
};

export const antdThemeConfig = computed(() => {
  if (!theme.isDark) {
    return { algorithm: antdTheme.defaultAlgorithm };
  }
  return {
    algorithm: antdTheme.darkAlgorithm,
    token: theme.isUltra ? ULTRA_DARK_TOKENS : DARK_TOKENS,
    components: {
      Layout: theme.isUltra ? ULTRA_DARK_LAYOUT_TOKENS : DARK_LAYOUT_TOKENS,
      Menu: theme.isUltra ? ULTRA_DARK_MENU_TOKENS : DARK_MENU_TOKENS,
    },
  };
});

export function toggleTheme() {
  theme.isDark = !theme.isDark;
}

export function toggleUltra() {
  theme.isUltra = !theme.isUltra;
}

// Briefly disable theme transition animations while a toggle is in
// flight, then re-enable on mouseleave. Mirrors the legacy panel's
// behavior of preventing flicker when hovering the theme menu.
export function pauseAnimationsUntilLeave(elementId) {
  document.documentElement.setAttribute('data-theme-animations', 'off');
  const el = document.getElementById(elementId);
  if (!el) return;
  const restore = () => {
    document.documentElement.removeAttribute('data-theme-animations');
    el.removeEventListener('mouseleave', restore);
    el.removeEventListener('touchend', restore);
  };
  el.addEventListener('mouseleave', restore);
  el.addEventListener('touchend', restore);
}

// Apply theme to DOM and persist whenever it changes.
watchEffect(() => {
  document.body.setAttribute('class', theme.isDark ? 'dark' : 'light');
  localStorage.setItem(STORAGE_DARK, String(theme.isDark));

  if (theme.isUltra) {
    document.documentElement.setAttribute('data-theme', 'ultra-dark');
  } else {
    document.documentElement.removeAttribute('data-theme');
  }
  localStorage.setItem(STORAGE_ULTRA, String(theme.isUltra));

  // Keep the global #message container's class in sync so AD-Vue toasts
  // pick up the right styling.
  const msg = document.getElementById('message');
  if (msg) msg.className = theme.isDark ? 'dark' : 'light';
});
