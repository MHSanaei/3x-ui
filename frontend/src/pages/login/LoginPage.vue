<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { UserOutlined, LockOutlined, KeyOutlined, SettingOutlined } from '@ant-design/icons-vue';
import { theme as antdTheme } from 'ant-design-vue';

import { HttpUtil } from '@/utils';
import { currentTheme, theme as themeState } from '@/composables/useTheme.js';
import ThemeSwitchLogin from '@/components/ThemeSwitchLogin.vue';

// Drive AD-Vue 4's built-in dark algorithm from our useTheme state.
// This re-themes every AD-Vue component without depending on the
// legacy panel's custom.min.css.
const antdThemeConfig = computed(() => ({
  algorithm: themeState.isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
}));

// Cycle the title between "Hello" and "Welcome" — matches the legacy
// panel's Vue 2 .is-visible / .is-hidden DOM-class swap, but driven
// reactively + with a Vue 3 <Transition> for the fade.
const HEADLINE_WORDS = ['Hello', 'Welcome'];
const HEADLINE_INTERVAL_MS = 2000;
const headlineIndex = ref(0);
let headlineTimer = null;

onMounted(() => {
  headlineTimer = window.setInterval(() => {
    headlineIndex.value = (headlineIndex.value + 1) % HEADLINE_WORDS.length;
  }, HEADLINE_INTERVAL_MS);
});

onBeforeUnmount(() => {
  if (headlineTimer != null) window.clearInterval(headlineTimer);
});

// Phase 4 ships this page in English only. Translations come back in
// Phase 7 (vue-i18n) once we decide how the new build pipeline reads
// the existing TOML translation files.

const fetched = ref(false);
const submitting = ref(false);
const twoFactorEnable = ref(false);

const user = reactive({
  username: '',
  password: '',
  twoFactorCode: '',
});

// In production the Go panel will inject a base path; during `npm run dev`
// we hit Vite's dev server and the configured proxy routes /login, /panel,
// etc. to the local Go backend.
const basePath = window.__X_UI_BASE_PATH__ || '';

onMounted(async () => {
  const msg = await HttpUtil.post('/getTwoFactorEnable');
  if (msg.success) {
    twoFactorEnable.value = !!msg.obj;
  }
  fetched.value = true;
});

async function login() {
  submitting.value = true;
  try {
    const msg = await HttpUtil.post('/login', user);
    if (msg.success) {
      window.location.href = basePath + 'panel/';
    }
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <a-config-provider :theme="antdThemeConfig">
  <a-layout class="login-app" :class="{ 'is-dark': themeState.isDark, 'is-ultra': themeState.isUltra }">
    <a-layout-content class="login-content">
      <div class="waves-header">
        <div class="waves-inner-header"></div>
        <svg class="waves" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
          viewBox="0 24 150 28" preserveAspectRatio="none" shape-rendering="auto">
          <defs>
            <path id="gentle-wave" d="M-160 44c30 0 58-18 88-18s 58 18 88 18 58-18 88-18 58 18 88 18 v44h-352z" />
          </defs>
          <g class="parallax">
            <use xlink:href="#gentle-wave" x="48" y="0" />
            <use xlink:href="#gentle-wave" x="48" y="3" />
            <use xlink:href="#gentle-wave" x="48" y="5" />
            <use xlink:href="#gentle-wave" x="48" y="7" />
          </g>
        </svg>
      </div>

      <a-row type="flex" justify="center" align="middle" class="login-row">
        <a-col :xs="22" :sm="14" :md="10" :lg="8" :xl="6" class="login-card">
          <div v-if="!fetched" class="login-loading">
            <a-spin size="large" />
          </div>

          <div v-else>
            <div class="login-settings">
              <a-popover :overlay-class-name="currentTheme" title="Settings" placement="bottomRight" trigger="click">
                <template #content>
                  <ThemeSwitchLogin />
                </template>
                <a-button shape="circle">
                  <template #icon><SettingOutlined /></template>
                </a-button>
              </a-popover>
            </div>

            <a-row justify="center">
              <a-col :span="24">
                <h2 class="login-title">
                  <Transition name="headline" mode="out-in">
                    <b :key="headlineIndex">{{ HEADLINE_WORDS[headlineIndex] }}</b>
                  </Transition>
                </h2>
              </a-col>
            </a-row>

            <a-form layout="vertical" @submit.prevent="login">
              <a-form-item>
                <a-input
                  v-model:value="user.username"
                  autocomplete="username"
                  name="username"
                  placeholder="Username"
                  autofocus
                  required
                >
                  <template #prefix><UserOutlined /></template>
                </a-input>
              </a-form-item>

              <a-form-item>
                <a-input-password
                  v-model:value="user.password"
                  autocomplete="current-password"
                  name="password"
                  placeholder="Password"
                  required
                >
                  <template #prefix><LockOutlined /></template>
                </a-input-password>
              </a-form-item>

              <a-form-item v-if="twoFactorEnable">
                <a-input
                  v-model:value="user.twoFactorCode"
                  autocomplete="one-time-code"
                  name="twoFactorCode"
                  placeholder="Two-factor code"
                  required
                >
                  <template #prefix><KeyOutlined /></template>
                </a-input>
              </a-form-item>

              <a-form-item>
                <a-row justify="center">
                  <a-button type="primary" html-type="submit" :loading="submitting" block>
                    {{ submitting ? '' : 'Login' }}
                  </a-button>
                </a-row>
              </a-form-item>
            </a-form>
          </div>
        </a-col>
      </a-row>
    </a-layout-content>
  </a-layout>
  </a-config-provider>
</template>

<style scoped>
/* Page palette comes straight from the legacy panel's CSS variables
 * (web/assets/css/custom.min.css). Driving everything off CSS vars
 * means the .is-dark / .is-ultra class swap is a one-liner.
 *
 * Wave layout, faithfully matching the legacy:
 *   - .waves-inner-header: 50vh of solid color
 *   - .waves SVG: 15vh of animated wave below it
 *   - Together they form a 65vh-tall colored region anchored to the top,
 *     with the form floating centered on top of it. */
.login-app {
  /* Light mode mirrors the legacy: the wave-header (top ~65vh) is the
   * lighter mint #dbf5ed, the rest of the page (--bg-page) is the
   * slightly darker mint #c7ebe2 — the bottom wave fill is the same
   * color so the wave reads as a continuation of the page bg. */
  --bg-page: #c7ebe2;
  --bg-wave-header: #dbf5ed;
  --bg-card: #ffffff;
  --color-title: #008771;
  --shadow-card: 0 2px 8px rgba(0, 0, 0, 0.09);
  --wave-fill: rgba(0, 135, 113, 0.12);
  --wave-fill-bottom: #c7ebe2;

  min-height: 100vh;
}

.login-app.is-dark {
  --bg-page: #222d42;        /* legacy .dark .under = surface-200 */
  --bg-wave-header: #0a1222; /* legacy --dark-color-background (login-bg defaults to this) */
  --bg-card: #151f31;        /* legacy surface-100 */
  --color-title: rgba(255, 255, 255, 0.92);
  --shadow-card: 0 4px 16px rgba(0, 0, 0, 0.45);
  --wave-fill: #222d42;
  --wave-fill-bottom: #222d42;
}

.login-app.is-dark.is-ultra {
  --bg-page: #0f2d32;        /* legacy ultra .under = login-wave override */
  --bg-wave-header: #0a2227; /* legacy ultra --dark-color-login-background */
  --bg-card: #0c0e12;        /* legacy ultra surface-100 */
  /* Legacy ultra-dark uses #0f2d32 for both wave-fill and bg-page,
   * which leaves near-zero contrast against #0a2227 and the wave
   * reads as static. Bump to a noticeably lighter teal so motion is
   * visible — every other value stays legacy-true. */
  --wave-fill: #1f4d52;
  --wave-fill-bottom: #1f4d52;
}

/* Both ant-layout and ant-layout-content default to opaque backgrounds.
 * Force them transparent so the page-bg painted on .login-app shows
 * through, and so the fixed waves-header isn't covered by the layout. */
.login-app,
.login-app :deep(.ant-layout-content) {
  background: transparent;
}
.login-app {
  background: var(--bg-page);
}

.login-card {
  background: var(--bg-card);
  box-shadow: var(--shadow-card);
}

.login-title {
  color: var(--color-title);
}

.login-settings {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 8px;
}

.login-content {
  position: relative;
}

/* Form sits above the fixed wave-header (which is at z-index: 0). */
.login-row {
  position: relative;
  z-index: 1;
  min-height: 100vh;
  padding: 24px 0;
}

.login-card {
  border-radius: 2rem;
  padding: 4rem 3rem;
  transition: background 0.3s, box-shadow 0.3s;
}

.login-loading {
  text-align: center;
  padding: 40px 0;
}

.login-title {
  text-align: center;
  margin-bottom: 32px;
  font-size: 2rem;
  font-weight: 500;
  min-height: 2.5rem;
}

.login-title b {
  display: inline-block;
}

/* Cycle word fade — analogous to the legacy .is-visible / .is-hidden
 * classes, but using Vue 3's <Transition> so we don't have to manage
 * the DOM by hand. */
.headline-enter-active,
.headline-leave-active {
  transition: opacity 0.4s ease, transform 0.4s ease;
}
.headline-enter-from {
  opacity: 0;
  transform: translateY(-12px);
}
.headline-leave-to {
  opacity: 0;
  transform: translateY(12px);
}

.waves-header {
  position: fixed;
  inset: 0 0 auto 0;
  width: 100%;
  z-index: 0;
  pointer-events: none;
  background: var(--bg-wave-header);
}

.waves-inner-header {
  height: 50vh;
  width: 100%;
}

.waves {
  position: relative;
  display: block;
  width: 100%;
  height: 15vh;
  min-height: 100px;
  max-height: 150px;
  margin-bottom: -8px;
}

/* Wave fills are CSS-driven so they switch with the theme; legacy used
 * inline fill="..." on each <use> which made them lock to one palette.
 * Animation durations match the legacy (4s/7s/10s/13s) so the bottom
 * wave actually visibly moves in dark mode where contrast is low. */
.parallax > use {
  fill: var(--wave-fill);
  animation: move-forever 25s cubic-bezier(0.55, 0.5, 0.45, 0.5) infinite;
}
.parallax > use:nth-child(1) { animation-delay: -2s; animation-duration: 4s;  opacity: 0.2; }
.parallax > use:nth-child(2) { animation-delay: -3s; animation-duration: 7s;  opacity: 0.4; }
.parallax > use:nth-child(3) { animation-delay: -4s; animation-duration: 10s; opacity: 0.6; }
.parallax > use:nth-child(4) {
  animation-delay: -5s;
  animation-duration: 13s;
  fill: var(--wave-fill-bottom);
  opacity: 1;
}

@keyframes move-forever {
  0% { transform: translate3d(-90px, 0, 0); }
  100% { transform: translate3d(85px, 0, 0); }
}
</style>
