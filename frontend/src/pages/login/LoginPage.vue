<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { UserOutlined, LockOutlined, KeyOutlined, SettingOutlined } from '@ant-design/icons-vue';

import { HttpUtil, LanguageManager } from '@/utils';
import {
  antdThemeConfig,
  currentTheme,
  theme as themeState,
} from '@/composables/useTheme.js';
import ThemeSwitchLogin from '@/components/ThemeSwitchLogin.vue';

const { t } = useI18n();

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

const fetched = ref(false);
const submitting = ref(false);
const twoFactorEnable = ref(false);

const user = reactive({
  username: '',
  password: '',
  twoFactorCode: '',
});

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

const lang = ref(LanguageManager.getLanguage());
function onLangChange(next) {
  LanguageManager.setLanguage(next);
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
          <a-col class="login-card">
            <div v-if="!fetched" class="login-loading">
              <a-spin size="large" />
            </div>

            <div v-else>
              <div class="login-settings">
                <a-popover :overlay-class-name="currentTheme" :title="t('menu.settings')" placement="bottomRight"
                  trigger="click">
                  <template #content>
                    <a-space direction="vertical" :size="10" class="settings-popover">
                      <ThemeSwitchLogin />
                      <span>{{ t('pages.settings.language') }}</span>
                      <a-select v-model:value="lang" class="lang-select" @change="onLangChange">
                        <a-select-option v-for="l in LanguageManager.supportedLanguages" :key="l.value"
                          :value="l.value">
                          <span :aria-label="l.name">{{ l.icon }}</span>
                          &nbsp;&nbsp;<span>{{ l.name }}</span>
                        </a-select-option>
                      </a-select>
                    </a-space>
                  </template>
                  <a-button shape="circle">
                    <template #icon>
                      <SettingOutlined />
                    </template>
                  </a-button>
                </a-popover>
              </div>

              <a-row justify="center">
                <a-col :span="24">
                  <h2 class="login-title">
                    <Transition name="headline" mode="out-in">
                      <b :key="headlineIndex">{{ headlineWords[headlineIndex] }}</b>
                    </Transition>
                  </h2>
                </a-col>
              </a-row>

              <a-form layout="vertical" @submit.prevent="login">
                <a-form-item>
                  <a-input v-model:value="user.username" autocomplete="username" name="username"
                    :placeholder="t('username')" autofocus required>
                    <template #prefix>
                      <UserOutlined />
                    </template>
                  </a-input>
                </a-form-item>

                <a-form-item>
                  <a-input-password v-model:value="user.password" autocomplete="current-password" name="password"
                    :placeholder="t('password')" required>
                    <template #prefix>
                      <LockOutlined />
                    </template>
                  </a-input-password>
                </a-form-item>

                <a-form-item v-if="twoFactorEnable">
                  <a-input v-model:value="user.twoFactorCode" autocomplete="one-time-code" name="twoFactorCode"
                    :placeholder="t('twoFactorCode')" required>
                    <template #prefix>
                      <KeyOutlined />
                    </template>
                  </a-input>
                </a-form-item>

                <a-form-item>
                  <a-row justify="center">
                    <a-button type="primary" html-type="submit" :loading="submitting" block>
                      {{ submitting ? '' : t('login') }}
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
.login-app {
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
  --bg-page: #222d42;
  --bg-wave-header: #0a1222;
  --bg-card: #151f31;
  --color-title: rgba(255, 255, 255, 0.92);
  --shadow-card: 0 4px 16px rgba(0, 0, 0, 0.45);
  --wave-fill: #222d42;
  --wave-fill-bottom: #222d42;
}

.login-app.is-dark.is-ultra {
  --bg-page: #0f2d32;
  --bg-wave-header: #0a2227;
  --bg-card: #0c0e12;
  --wave-fill: #1f4d52;
  --wave-fill-bottom: #0f2d32;
}

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

.settings-popover {
  min-width: 220px;
}

.lang-select {
  width: 100%;
}

.login-content {
  position: relative;
}

.login-row {
  position: relative;
  z-index: 1;
  min-height: 100vh;
  padding: 24px 0;
}

.login-card {
  width: clamp(280px, 90vw, 300px);
  border-radius: 2rem;
  padding: clamp(2rem, 5vw, 4rem) 1.5rem;
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

.parallax>use {
  fill: var(--wave-fill);
  animation: move-forever 25s cubic-bezier(0.55, 0.5, 0.45, 0.5) infinite;
}

.parallax>use:nth-child(1) {
  animation-delay: -2s;
  animation-duration: 4s;
  opacity: 0.2;
}

.parallax>use:nth-child(2) {
  animation-delay: -3s;
  animation-duration: 7s;
  opacity: 0.4;
}

.parallax>use:nth-child(3) {
  animation-delay: -4s;
  animation-duration: 10s;
  opacity: 0.6;
}

.parallax>use:nth-child(4) {
  animation-delay: -5s;
  animation-duration: 13s;
  fill: var(--wave-fill-bottom);
  opacity: 1;
}

@keyframes move-forever {
  0% {
    transform: translate3d(-90px, 0, 0);
  }

  100% {
    transform: translate3d(85px, 0, 0);
  }
}
</style>
