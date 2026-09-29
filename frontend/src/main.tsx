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

setupHttp();

// A tab opened before an upgrade still points at chunk names the new build no
// longer serves; reload once to pick up the new build instead of an error page.
window.addEventListener('vite:preloadError', (event) => {
  try {
    if (sessionStorage.getItem('chunk-reloaded')) return;
    sessionStorage.setItem('chunk-reloaded', '1');
  } catch {
    return;
  }
  event.preventDefault();
  window.location.reload();
});

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
