import { createApp } from 'vue';
import Antd, { message } from 'ant-design-vue';
import 'ant-design-vue/dist/reset.css';

import { setupAxios } from '@/api/axios-init.js';
import '@/composables/useTheme.js';
import { i18n, readyI18n } from '@/i18n/index.js';
import { applyDocumentTitle } from '@/utils';
import InboundsPage from '@/pages/inbounds/InboundsPage.vue';

setupAxios();
applyDocumentTitle();

const messageContainer = document.getElementById('message');
if (messageContainer) {
  message.config({ getContainer: () => messageContainer });
}

readyI18n().then(() => {
  createApp(InboundsPage).use(Antd).use(i18n).mount('#app');
});
