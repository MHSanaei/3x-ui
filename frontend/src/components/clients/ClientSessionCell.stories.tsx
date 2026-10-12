import type { Meta, StoryObj } from '@storybook/react-vite';

import { ThemeProvider } from '@/hooks/useTheme';

import ClientSessionCell from './ClientSessionCell';

const MiB = 1024 ** 2;
const sessionStart = Date.UTC(2026, 9, 5, 9, 15);

const meta = {
  title: 'Clients/ClientSessionCell',
  component: ClientSessionCell,
  tags: ['autodocs'],
  decorators: [
    (Story) => (
      <ThemeProvider>
        <Story />
      </ThemeProvider>
    ),
  ],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          "Traffic a client moved during its latest online session: blue like the live speed tag while the session is ongoing, with an upload/download breakdown and the session's start (and end, once it is over) in a hover popover. A dash means no session has been recorded yet.",
      },
    },
  },
  argTypes: {
    up: { description: 'Bytes uploaded during the session.' },
    down: { description: 'Bytes downloaded during the session.' },
    start: { description: 'Session start in Unix milliseconds; 0 means no session recorded yet.' },
    lastOnline: {
      description:
        "Client's last activity in Unix milliseconds, shown as the end of a session that is over.",
    },
    ongoing: { description: 'The client is online now, so this is its current session.' },
  },
} satisfies Meta<typeof ClientSessionCell>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Ongoing: Story = {
  args: {
    up: 48 * MiB,
    down: 612 * MiB,
    start: sessionStart,
    lastOnline: sessionStart + 95 * 60_000,
    ongoing: true,
  },
};

export const Ended: Story = {
  args: {
    up: 12 * MiB,
    down: 230 * MiB,
    start: sessionStart,
    lastOnline: sessionStart + 40 * 60_000,
  },
};

export const NoSession: Story = {
  args: {
    lastOnline: sessionStart,
  },
};
