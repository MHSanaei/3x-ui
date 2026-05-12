import { createApp } from 'vue';
import Antd, { message } from 'ant-design-vue';
import 'ant-design-vue/dist/reset.css';

import { setupAxios } from '@/api/axios-init.js';
// Importing this module triggers the boot side-effect that applies the
// stored theme to <body>/<html> before Vue renders anything.
import '@/composables/useTheme.js';
import { i18n } from '@/i18n/index.js';
import LoginPage from '@/pages/login/LoginPage.vue';

setupAxios();

// Toasts attach to a #message div the page provides — keeps theme
// styling in sync with the rest of the panel.
const messageContainer = document.getElementById('message');
if (messageContainer) {
  message.config({ getContainer: () => messageContainer });
}

createApp(LoginPage).use(Antd).use(i18n).mount('#app');
