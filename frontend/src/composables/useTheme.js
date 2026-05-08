import { reactive, computed, watchEffect } from 'vue';

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
