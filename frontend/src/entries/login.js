import { createApp } from 'vue';
import Antd, { message } from 'ant-design-vue';
import 'ant-design-vue/dist/reset.css';

import { setupAxios } from '@/api/axios-init.js';
// Importing this module triggers the boot side-effect that applies the
// stored theme to <body>/<html> before Vue renders anything.
import '@/composables/useTheme.js';
import { i18n, readyI18n } from '@/i18n/index.js';
import { applyDocumentTitle } from '@/utils';
import LoginPage from '@/pages/login/LoginPage.vue';

setupAxios();
applyDocumentTitle();

const messageContainer = document.getElementById('message');
if (messageContainer) {
  message.config({ getContainer: () => messageContainer });
}

readyI18n().then(() => {
  createApp(LoginPage).use(Antd).use(i18n).mount('#app');
});
