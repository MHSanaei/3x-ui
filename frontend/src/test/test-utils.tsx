import type { ReactElement } from 'react';
import { render } from '@testing-library/react';

import { ThemeProvider } from '@/hooks/useTheme';

export function renderWithProviders(ui: ReactElement) {
  return render(<ThemeProvider>{ui}</ThemeProvider>);
}

export function fieldLabels(): string[] {
  return Array.from(document.querySelectorAll('.ant-form-item-label label'))
    .map((el) => (el.textContent ?? '').trim())
    .filter(Boolean);
}
