import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router/dom';
import { message } from 'antd';
import 'antd/dist/reset.css';
import '@/styles/utils.css';
import '@/styles/page-shell.css';
import '@/styles/page-cards.css';

import { setupHttp } from '@/api/http-init';
import { readyI18n } from '@/i18n/react';
import { ThemeProvider } from '@/hooks/useTheme';
import { QueryProvider } from '@/api/QueryProvider';
import { router } from '@/routes';

// A stale tab keeps this entry's old chunk URLs after a deploy. Reload once per
// panel base path and entry URL; the same bundle must not loop on a real outage.
const chunkRecoveryKey = `xui:chunk-recovery:${window.X_UI_BASE_PATH || '/'}:${import.meta.url}`;
let chunkRecoveryCommitted = false;

window.addEventListener('vite:preloadError', (event) => {
  if (chunkRecoveryCommitted) return;
  chunkRecoveryCommitted = true;
  let shouldReload = false;
  try {
    if (sessionStorage.getItem(chunkRecoveryKey) == null) {
      sessionStorage.setItem(chunkRecoveryKey, '1');
      shouldReload = true;
    }
  } catch {
    chunkRecoveryCommitted = false;
    return;
  }
  if (!shouldReload) {
    chunkRecoveryCommitted = false;
    return;
  }
  event.preventDefault();
  location.reload();
});

setupHttp();

const messageContainer = document.getElementById('message');
if (messageContainer) {
  message.config({ getContainer: () => messageContainer });
}

readyI18n().then(() => {
  const root = document.getElementById('app');
  if (root) {
    createRoot(root).render(
      <ThemeProvider>
        <QueryProvider>
          <RouterProvider router={router} />
        </QueryProvider>
      </ThemeProvider>,
    );
  }
});
