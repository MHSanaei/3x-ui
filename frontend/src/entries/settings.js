import { createApp } from 'vue';
import Antd, { message } from 'ant-design-vue';
import 'ant-design-vue/dist/reset.css';

import { setupAxios } from '@/api/axios-init.js';
// Importing useTheme triggers the boot side-effect that applies the
// stored theme to <body>/<html> before Vue mounts.
import '@/composables/useTheme.js';
import { i18n } from '@/i18n/index.js';
import { applyDocumentTitle } from '@/utils';
import SettingsPage from '@/pages/settings/SettingsPage.vue';

setupAxios();
applyDocumentTitle();

const messageContainer = document.getElementById('message');
if (messageContainer) {
  message.config({ getContainer: () => messageContainer });
}

createApp(SettingsPage).use(Antd).use(i18n).mount('#app');
