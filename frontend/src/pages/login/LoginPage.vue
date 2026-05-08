<script setup>
import { onMounted, reactive, ref } from 'vue';
import { UserOutlined, LockOutlined, KeyOutlined } from '@ant-design/icons-vue';

import { HttpUtil } from '@/utils';

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
  <a-layout class="login-app">
    <a-layout-content class="login-content">
      <div class="waves-header">
        <svg class="waves" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
          viewBox="0 24 150 28" preserveAspectRatio="none" shape-rendering="auto">
          <defs>
            <path id="gentle-wave" d="M-160 44c30 0 58-18 88-18s 58 18 88 18 58-18 88-18 58 18 88 18 v44h-352z" />
          </defs>
          <g class="parallax">
            <use xlink:href="#gentle-wave" x="48" y="0" fill="rgba(0, 135, 113, 0.08)" />
            <use xlink:href="#gentle-wave" x="48" y="3" fill="rgba(0, 135, 113, 0.08)" />
            <use xlink:href="#gentle-wave" x="48" y="5" fill="rgba(0, 135, 113, 0.08)" />
            <use xlink:href="#gentle-wave" x="48" y="7" fill="#c7ebe2" />
          </g>
        </svg>
      </div>

      <a-row type="flex" justify="center" align="middle" class="login-row">
        <a-col :xs="22" :sm="14" :md="10" :lg="8" :xl="6" class="login-card">
          <div v-if="!fetched" class="login-loading">
            <a-spin size="large" />
          </div>

          <div v-else>
            <a-row justify="center">
              <a-col :span="24">
                <h2 class="login-title">Welcome to 3x-ui</h2>
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
</template>

<style scoped>
.login-app {
  min-height: 100vh;
  background: #f0f2f5;
}

.login-content {
  position: relative;
}

.login-row {
  min-height: 100vh;
  padding: 24px 0;
}

.login-card {
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  padding: 40px 32px;
}

.login-loading {
  text-align: center;
  padding: 40px 0;
}

.login-title {
  text-align: center;
  margin-bottom: 32px;
  color: #008771;
  font-size: 24px;
  font-weight: 500;
}

.waves-header {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  pointer-events: none;
  overflow: hidden;
  height: 200px;
}

.waves {
  width: 100%;
  height: 100%;
  display: block;
}

.parallax > use {
  animation: move-forever 15s cubic-bezier(0.55, 0.5, 0.45, 0.5) infinite;
}

.parallax > use:nth-child(1) { animation-delay: -2s; animation-duration: 7s; }
.parallax > use:nth-child(2) { animation-delay: -3s; animation-duration: 10s; }
.parallax > use:nth-child(3) { animation-delay: -4s; animation-duration: 13s; }
.parallax > use:nth-child(4) { animation-delay: -5s; animation-duration: 20s; }

@keyframes move-forever {
  0% { transform: translate3d(-90px, 0, 0); }
  100% { transform: translate3d(85px, 0, 0); }
}
</style>
