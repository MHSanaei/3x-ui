import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import type { Sponsor } from '@/generated/types';
import SponsorCard from './SponsorCard';

const sponsor: Sponsor = {
  id: 'acme-2026-10',
  name: 'Acme VPS',
  slots: ['dashboard', 'sidebar', 'page', 'login'],
  until: '2099-01-01T00:00:00Z',
  title: { en: 'Acme VPS — fast NVMe servers', fa: 'سرورهای سریع Acme' },
  text: { en: 'Deploy 3X-UI in 60 seconds. 20% off for panel users.' },
  link: 'https://acme.example/?utm_source=3x-ui',
};

const meta = {
  title: 'Sponsor/SponsorCard',
  component: SponsorCard,
  tags: ['autodocs'],
  parameters: {
    docs: {
      description: {
        component:
          'Paid sponsor placement fed by the project sponsors.json. Always labelled as a sponsor; links open in a new tab with rel="sponsored".',
      },
    },
  },
  argTypes: {
    sponsor: { description: 'Sponsor entry from GET /sponsors.' },
    variant: {
      description: 'banner (dashboard), compact (sidebar/login) or card (Sponsors page).',
    },
    iconOnly: { description: 'Logo-only rendering for the collapsed sidebar rail.' },
    onClose: { description: 'When set, shows a close button (temporary dismiss).' },
  },
  args: { sponsor },
} satisfies Meta<typeof SponsorCard>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Banner: Story = {
  args: { variant: 'banner', onClose: () => {} },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByText('Sponsor')).toBeInTheDocument();
    await expect(canvas.getByRole('link')).toHaveAttribute('rel', 'noopener noreferrer sponsored');
  },
};

export const Compact: Story = {
  args: { variant: 'compact', onClose: () => {} },
  render: (args) => (
    <div style={{ width: 204 }}>
      <SponsorCard {...args} />
    </div>
  ),
};

export const Card: Story = {
  args: { variant: 'card' },
  render: (args) => (
    <div style={{ width: 320 }}>
      <SponsorCard {...args} />
    </div>
  ),
};

export const IconOnly: Story = {
  args: { iconOnly: true },
};
