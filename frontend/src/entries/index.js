import { createApp } from 'vue';
import Antd, { message } from 'ant-design-vue';
import 'ant-design-vue/dist/reset.css';

import { setupAxios } from '@/api/axios-init.js';
// Importing useTheme triggers the boot side-effect that applies the
// stored theme to <body>/<html> before Vue mounts.
import '@/composables/useTheme.js';
import { i18n } from '@/i18n/index.js';
import IndexPage from '@/pages/index/IndexPage.vue';

setupAxios();

const messageContainer = document.getElementById('message');
if (messageContainer) {
  message.config({ getContainer: () => messageContainer });
}

createApp(IndexPage).use(Antd).use(i18n).mount('#app');
