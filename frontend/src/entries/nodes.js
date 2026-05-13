import { createApp } from 'vue';
import Antd, { message } from 'ant-design-vue';
import 'ant-design-vue/dist/reset.css';

import { setupAxios } from '@/api/axios-init.js';
import '@/composables/useTheme.js';
import { i18n } from '@/i18n/index.js';
import { applyDocumentTitle } from '@/utils';
import NodesPage from '@/pages/nodes/NodesPage.vue';

setupAxios();
applyDocumentTitle();

const messageContainer = document.getElementById('message');
if (messageContainer) {
  message.config({ getContainer: () => messageContainer });
}

createApp(NodesPage).use(Antd).use(i18n).mount('#app');
