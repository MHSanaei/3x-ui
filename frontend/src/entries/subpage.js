import { createApp } from 'vue';
import Antd, { message } from 'ant-design-vue';
import 'ant-design-vue/dist/reset.css';

// The sub page is served by the subscription HTTP server (sub/sub.go)
// at /<linksPath>/<subId>?html=1. Go injects window.__SUB_PAGE_DATA__
// with the parsed traffic/quota/expiry view-model and the rendered
// share links — the SPA reads those at mount.
import '@/composables/useTheme.js';
import { i18n, readyI18n } from '@/i18n/index.js';
import SubPage from '@/pages/sub/SubPage.vue';

const messageContainer = document.getElementById('message');
if (messageContainer) {
  message.config({ getContainer: () => messageContainer });
}

readyI18n().then(() => {
  createApp(SubPage).use(Antd).use(i18n).mount('#app');
});
