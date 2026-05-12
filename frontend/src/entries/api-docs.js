import { createApp } from 'vue';
import Antd, { message } from 'ant-design-vue';
import 'ant-design-vue/dist/reset.css';

import { setupAxios } from '@/api/axios-init.js';
import '@/composables/useTheme.js';
import { i18n } from '@/i18n/index.js';
import ApiDocsPage from '@/pages/api-docs/ApiDocsPage.vue';

setupAxios();

const messageContainer = document.getElementById('message');
if (messageContainer) {
  message.config({ getContainer: () => messageContainer });
}

createApp(ApiDocsPage).use(Antd).use(i18n).mount('#app');
