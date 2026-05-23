import { createRoot } from 'react-dom/client';
import { message } from 'antd';
import 'antd/dist/reset.css';

import { setupAxios } from '@/api/axios-init.js';
import { applyDocumentTitle } from '@/utils';
import { readyI18n } from '@/i18n/react';
import { ThemeProvider } from '@/hooks/useTheme';
import IndexPage from '@/pages/index/IndexPage';

setupAxios();
applyDocumentTitle();

const messageContainer = document.getElementById('message');
if (messageContainer) {
  message.config({ getContainer: () => messageContainer });
}

readyI18n().then(() => {
  const root = document.getElementById('app');
  if (root) {
    createRoot(root).render(
      <ThemeProvider>
        <IndexPage />
      </ThemeProvider>,
    );
  }
});
