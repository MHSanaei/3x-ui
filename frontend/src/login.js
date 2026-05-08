import { createApp } from 'vue';
import Antd from 'ant-design-vue';
import 'ant-design-vue/dist/reset.css';

import { setupAxios } from '@/api/axios-init.js';
import LoginPage from '@/pages/login/LoginPage.vue';

setupAxios();

createApp(LoginPage).use(Antd).mount('#app');
