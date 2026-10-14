import { useEffect, useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router';
import { expect, waitFor, within } from 'storybook/test';

import { ThemeProvider } from '@/hooks/useTheme';
import CommandPalette from './CommandPalette';
import { commandPaletteStore } from './useCommandPalette';

let visibilityRead = false;

function PalettePreview() {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  useEffect(() => {
    commandPaletteStore.open();
    return () => commandPaletteStore.close();
  }, []);
  return (
    <QueryClientProvider client={client}>
      <ThemeProvider>
        <MemoryRouter>
          <CommandPalette />
        </MemoryRouter>
      </ThemeProvider>
    </QueryClientProvider>
  );
}

const meta = {
  title: 'Navigation/CommandPalette',
  component: CommandPalette,
  tags: ['autodocs'],
  parameters: {
    sponsorsEnabled: true,
    docs: {
      description: {
        component:
          'Navigation and actions. The Sponsors command is available only when the public server setting allows sponsor content.',
      },
    },
  },
  beforeEach: (context) => {
    visibilityRead = false;
    const originalFetch = window.fetch;
    window.fetch = async (input, init) => {
      const url = new URL(
        typeof input === 'string' ? input : input instanceof URL ? input.href : input.url,
        window.location.origin,
      );
      let obj: unknown;
      if (url.pathname === '/sponsors/enabled') {
        visibilityRead = true;
        obj = context.parameters.sponsorsEnabled;
      } else if (url.pathname === '/panel/api/setting/all') {
        obj = {};
      } else if (url.pathname === '/panel/api/inbounds/options') {
        obj = [];
      } else if (url.pathname === '/csrf-token') {
        obj = 'storybook-csrf';
      } else {
        return originalFetch(input, init);
      }
      return new Response(JSON.stringify({ success: true, msg: '', obj }), {
        headers: { 'content-type': 'application/json' },
      });
    };
    return () => {
      window.fetch = originalFetch;
    };
  },
  render: () => <PalettePreview />,
} satisfies Meta<typeof CommandPalette>;

export default meta;
type Story = StoryObj<typeof meta>;

export const SponsorsEnabled: Story = {
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).findByText('Sponsors')).resolves.toBeInTheDocument();
  },
};

export const SponsorsDisabled: Story = {
  parameters: { sponsorsEnabled: false },
  play: async ({ canvasElement }) => {
    await waitFor(() => expect(visibilityRead).toBe(true));
    await expect(within(canvasElement).queryByText('Sponsors')).not.toBeInTheDocument();
  },
};
