<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { UserOutlined, LockOutlined, KeyOutlined, SettingOutlined } from '@ant-design/icons-vue';

import { HttpUtil, LanguageManager } from '@/utils';
import {
  antdThemeConfig,
  currentTheme,
  theme as themeState,
  toggleTheme,
  toggleUltra,
  pauseAnimationsUntilLeave,
} from '@/composables/useTheme.js';

const { t } = useI18n();

const fetched = ref(false);
const submitting = ref(false);
const twoFactorEnable = ref(false);

const user = reactive({
  username: '',
  password: '',
  twoFactorCode: '',
});

const basePath = window.X_UI_BASE_PATH || '';

const headlineWords = computed(() => [t('pages.login.hello'), t('pages.login.title')]);
const HEADLINE_INTERVAL_MS = 2000;
const headlineIndex = ref(0);
let headlineTimer = null;

onMounted(() => {
  headlineTimer = window.setInterval(() => {
    headlineIndex.value = (headlineIndex.value + 1) % headlineWords.value.length;
  }, HEADLINE_INTERVAL_MS);
});

onBeforeUnmount(() => {
  if (headlineTimer != null) window.clearInterval(headlineTimer);
});

onMounted(async () => {
  const msg = await HttpUtil.post('/getTwoFactorEnable');
  if (msg.success) twoFactorEnable.value = !!msg.obj;
  fetched.value = true;
});

async function login() {
  submitting.value = true;
  try {
    const msg = await HttpUtil.post('/login', user);
    if (msg.success) window.location.href = basePath + 'panel/';
  } finally {
    submitting.value = false;
  }
}

const lang = ref(LanguageManager.getLanguage());
function onLangChange(next) {
  LanguageManager.setLanguage(next);
}

/* Same Light -> Dark -> Ultra Dark -> Light cycle the sidebar's brand
 * button uses, so the login chrome offers a one-click theme toggle
 * without the popover ceremony. */
function cycleTheme() {
  pauseAnimationsUntilLeave('login-theme-cycle');
  if (!themeState.isDark) {
    toggleTheme();
    if (themeState.isUltra) toggleUltra();
  } else if (!themeState.isUltra) {
    toggleUltra();
  } else {
    toggleUltra();
    toggleTheme();
  }
}
</script>

<template>
  <a-config-provider :theme="antdThemeConfig">
    <a-layout class="login-app" :class="{ 'is-dark': themeState.isDark, 'is-ultra': themeState.isUltra }">
      <a-layout-content class="login-content">
        <!-- Floating chrome at top-right: theme cycle (Light/Dark/Ultra)
             plus a language picker hidden behind the gear popover. -->
        <div class="login-toolbar">
          <button type="button" class="theme-cycle" :aria-label="t('menu.theme')" :title="t('menu.theme')"
            @click="cycleTheme">
            <svg v-if="!themeState.isDark" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
              stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <circle cx="12" cy="12" r="4" />
              <path
                d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
            </svg>
            <svg v-else-if="!themeState.isUltra" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
              stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
            </svg>
            <svg v-else viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="1.5"
              stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
              <path fill="none" d="M19 3l0.7 1.4 1.4 0.7-1.4 0.7L19 7.2l-0.7-1.4-1.4-0.7 1.4-0.7z" />
            </svg>
          </button>

          <a-popover :overlay-class-name="currentTheme" :title="t('pages.settings.language')" placement="bottomRight"
            trigger="click">
            <template #content>
              <a-space direction="vertical" :size="10" class="settings-popover">
                <a-select v-model:value="lang" class="lang-select" @change="onLangChange">
                  <a-select-option v-for="l in LanguageManager.supportedLanguages" :key="l.value" :value="l.value">
                    <span :aria-label="l.name">{{ l.icon }}</span>
                    &nbsp;&nbsp;<span>{{ l.name }}</span>
                  </a-select-option>
                </a-select>
              </a-space>
            </template>
            <a-button shape="circle" class="toolbar-btn" :aria-label="t('menu.settings')">
              <template #icon>
                <SettingOutlined />
              </template>
            </a-button>
          </a-popover>
        </div>

        <div class="login-wrapper">
          <div v-if="!fetched" class="login-loading">
            <a-spin size="large" />
          </div>

          <div v-else class="login-card">
            <div class="brand">
              <span class="brand-name">3X-UI</span>
              <span class="brand-accent" aria-hidden="true"></span>
            </div>
            <h2 class="welcome">
              <Transition name="headline" mode="out-in">
                <b :key="headlineIndex">{{ headlineWords[headlineIndex] }}</b>
              </Transition>
            </h2>

            <a-form layout="vertical" class="login-form" @submit.prevent="login">
              <a-form-item :label="t('username')">
                <a-input v-model:value="user.username" autocomplete="username" name="username" size="large"
                  :placeholder="t('username')" autofocus required>
                  <template #prefix>
                    <UserOutlined />
                  </template>
                </a-input>
              </a-form-item>

              <a-form-item :label="t('password')">
                <a-input-password v-model:value="user.password" autocomplete="current-password" name="password"
                  size="large" :placeholder="t('password')" required>
                  <template #prefix>
                    <LockOutlined />
                  </template>
                </a-input-password>
              </a-form-item>

              <a-form-item v-if="twoFactorEnable" :label="t('twoFactorCode')">
                <a-input v-model:value="user.twoFactorCode" autocomplete="one-time-code" name="twoFactorCode"
                  size="large" :placeholder="t('twoFactorCode')" required>
                  <template #prefix>
                    <KeyOutlined />
                  </template>
                </a-input>
              </a-form-item>

              <a-form-item class="submit-row">
                <a-button type="primary" html-type="submit" :loading="submitting" size="large" block>
                  {{ submitting ? '' : t('login') }}
                </a-button>
              </a-form-item>
            </a-form>

          </div>
        </div>
      </a-layout-content>
    </a-layout>
  </a-config-provider>
</template>

<style scoped>
.login-app {
  --bg-page: #f5f7fa;
  --bg-card: #ffffff;
  --color-text: rgba(0, 0, 0, 0.88);
  --color-text-subtle: rgba(0, 0, 0, 0.55);
  --color-accent: #1677ff;
  --color-border: rgba(0, 0, 0, 0.08);
  --shadow-card: 0 1px 3px rgba(0, 0, 0, 0.04), 0 8px 24px rgba(0, 0, 0, 0.06);
  --blob-1: rgba(99, 102, 241, 0.45);
  --blob-2: rgba(236, 72, 153, 0.38);
  --blob-3: rgba(20, 184, 166, 0.32);

  position: relative;
  min-height: 100vh;
  overflow: hidden;
  background: var(--bg-page);
}

.login-app.is-dark {
  --bg-page: #1e1e1e;
  --bg-card: #252526;
  --color-text: rgba(255, 255, 255, 0.92);
  --color-text-subtle: rgba(255, 255, 255, 0.55);
  --color-accent: #4096ff;
  --color-border: rgba(255, 255, 255, 0.08);
  --shadow-card: 0 1px 3px rgba(0, 0, 0, 0.3), 0 8px 32px rgba(0, 0, 0, 0.4);
  --blob-1: rgba(64, 150, 255, 0.40);
  --blob-2: rgba(168, 85, 247, 0.34);
  --blob-3: rgba(34, 211, 238, 0.22);
}

.login-app.is-dark.is-ultra {
  --bg-page: #000;
  --bg-card: #141414;
  --color-border: rgba(255, 255, 255, 0.06);
  --blob-1: rgba(64, 150, 255, 0.22);
  --blob-2: rgba(168, 85, 247, 0.18);
  --blob-3: rgba(34, 211, 238, 0.12);
}

/* Three blurred blobs slowly drift across the page; ::before and
 * ::after carry two of them, the third lives on .login-content::before
 * so we can animate it independently. */
.login-app::before,
.login-app::after {
  content: '';
  position: absolute;
  width: 70vw;
  height: 70vw;
  max-width: 900px;
  max-height: 900px;
  border-radius: 50%;
  filter: blur(90px);
  pointer-events: none;
  z-index: 0;
  will-change: transform;
}

.login-app::before {
  top: -25vw;
  left: -20vw;
  background: radial-gradient(circle, var(--blob-1) 0%, transparent 65%);
  animation: blob-drift-a 24s ease-in-out infinite alternate;
}

.login-app::after {
  bottom: -25vw;
  right: -20vw;
  background: radial-gradient(circle, var(--blob-2) 0%, transparent 65%);
  animation: blob-drift-b 30s ease-in-out infinite alternate;
}

.login-content::before {
  content: '';
  position: absolute;
  top: 30%;
  left: 50%;
  width: 50vw;
  height: 50vw;
  max-width: 700px;
  max-height: 700px;
  border-radius: 50%;
  background: radial-gradient(circle, var(--blob-3) 0%, transparent 65%);
  filter: blur(90px);
  pointer-events: none;
  z-index: 0;
  will-change: transform;
  animation: blob-drift-c 36s ease-in-out infinite alternate;
}

@keyframes blob-drift-a {
  0% {
    transform: translate(0, 0) scale(1);
  }

  50% {
    transform: translate(18vw, 10vh) scale(1.15);
  }

  100% {
    transform: translate(34vw, 22vh) scale(1.25);
  }
}

@keyframes blob-drift-b {
  0% {
    transform: translate(0, 0) scale(1);
  }

  50% {
    transform: translate(-16vw, -10vh) scale(1.12);
  }

  100% {
    transform: translate(-30vw, -22vh) scale(1.2);
  }
}

@keyframes blob-drift-c {
  0% {
    transform: translate(-50%, -50%) scale(1);
  }

  50% {
    transform: translate(-20%, -20%) scale(1.1);
  }

  100% {
    transform: translate(-80%, -10%) scale(1.05);
  }
}

@media (prefers-reduced-motion: reduce) {

  .login-app::before,
  .login-app::after,
  .login-content::before {
    animation: none;
  }
}

.login-app :deep(.ant-layout-content) {
  background: transparent;
}

.login-content {
  position: relative;
}

.login-content>* {
  position: relative;
  z-index: 1;
}

.login-toolbar {
  position: fixed;
  top: 16px;
  right: 16px;
  z-index: 10;
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.toolbar-btn {
  width: 40px;
  height: 40px;
}

.theme-cycle {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  border: 1px solid var(--color-border);
  background: var(--bg-card);
  color: var(--color-text);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  padding: 0;
  transition: background-color 0.2s, transform 0.15s, color 0.2s;
}

.theme-cycle:hover,
.theme-cycle:focus-visible {
  background-color: rgba(64, 150, 255, 0.1);
  color: #4096ff;
  transform: scale(1.05);
  outline: none;
}

.theme-cycle svg {
  width: 18px;
  height: 18px;
}

.login-wrapper {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px 16px;
}

.login-loading {
  text-align: center;
}

.login-card {
  width: 100%;
  max-width: 400px;
  background: var(--bg-card);
  border: 1px solid var(--color-border);
  border-radius: 12px;
  padding: 40px 32px 28px;
  box-shadow: var(--shadow-card);
}

@media (max-width: 480px) {
  .login-card {
    padding: 32px 20px 24px;
  }
}

.brand {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.brand-name {
  font-size: 28px;
  font-weight: 700;
  letter-spacing: 1.5px;
  color: var(--color-text);
}

.brand-accent {
  display: block;
  width: 40px;
  height: 3px;
  border-radius: 2px;
  background: var(--color-accent);
}

.welcome {
  text-align: center;
  color: var(--color-text);
  font-size: 32px;
  font-weight: 700;
  line-height: 1.2;
  min-height: 42px;
  margin: 12px 0 28px;
  letter-spacing: 0.3px;
}

.welcome b {
  display: inline-block;
  font-weight: inherit;
}

.headline-enter-active,
.headline-leave-active {
  transition: opacity 280ms ease, transform 280ms ease;
}

.headline-enter-from {
  opacity: 0;
  transform: translateY(6px);
}

.headline-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

.login-form :deep(.ant-form-item-label > label) {
  color: var(--color-text);
  font-weight: 500;
}

.submit-row {
  margin-bottom: 0;
}

.settings-popover {
  min-width: 220px;
}

.lang-select {
  width: 100%;
}
</style>
