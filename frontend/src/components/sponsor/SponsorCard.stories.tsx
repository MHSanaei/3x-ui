import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import type { Sponsor } from '@/generated/types';
import SponsorCard from './SponsorCard';

const sponsor: Sponsor = {
  id: 'acme-2026-10',
  name: 'Acme VPS',
  slots: ['dashboard', 'page', 'login'],
  until: '2099-01-01T00:00:00Z',
  title: { en: 'Acme VPS — fast NVMe servers', fa: 'سرورهای سریع Acme' },
  text: { en: 'Deploy 3X-UI in 60 seconds. 20% off for panel users.' },
  link: 'https://acme.example/?utm_source=3x-ui',
};

// Copy as long as the real sponsors.json entries; each story renders it at its slot's width.
const longSponsor: Sponsor = {
  ...sponsor,
  title: { en: 'Advertise here — reach 3X-UI server admins' },
  text: {
    en: 'Put your VPS, hosting or domain brand in front of the admins who use 3X-UI every day. Message us for slots and pricing.',
  },
};

function part(root: HTMLElement, selector: string): HTMLElement {
  const el = root.querySelector<HTMLElement>(selector);
  if (!el) throw new Error(`missing ${selector}`);
  return el;
}

async function expectInsideCard(root: HTMLElement, selector: string) {
  const card = part(root, '.sponsor-card').getBoundingClientRect();
  const box = part(root, selector).getBoundingClientRect();
  await expect(box.left).toBeGreaterThanOrEqual(card.left - 0.5);
  await expect(box.right).toBeLessThanOrEqual(card.right + 0.5);
}

async function expectNotClipped(root: HTMLElement, selector: string) {
  const el = part(root, selector);
  await expect(el.scrollWidth).toBeLessThanOrEqual(el.clientWidth + 1);
  await expect(el.scrollHeight).toBeLessThanOrEqual(el.clientHeight + 1);
}

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
      description: 'banner (dashboard), compact (login) or card (Sponsors page).',
    },
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
    <div style={{ width: 334 }}>
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

export const BannerLongCopy: Story = {
  args: { variant: 'banner', sponsor: longSponsor, onClose: () => {} },
  render: (args) => (
    <div style={{ width: 360 }}>
      <SponsorCard {...args} />
    </div>
  ),
  play: async ({ canvasElement }) => {
    await expectNotClipped(canvasElement, '.sponsor-title');
  },
};

export const LoginLongCopy: Story = {
  args: { variant: 'compact', sponsor: longSponsor, onClose: () => {} },
  render: (args) => (
    <div style={{ width: 334 }}>
      <SponsorCard {...args} />
    </div>
  ),
  play: async ({ canvasElement }) => {
    await expectNotClipped(canvasElement, '.sponsor-title');
    await expectNotClipped(canvasElement, '.sponsor-text');
    await expect(part(canvasElement, '.sponsor-main')).toHaveAttribute(
      'title',
      `${longSponsor.title.en}\n${longSponsor.text.en}`,
    );
  },
};

export const PageLongCopy: Story = {
  args: { variant: 'card', sponsor: longSponsor },
  render: (args) => (
    <div style={{ width: 320 }}>
      <SponsorCard {...args} />
    </div>
  ),
  play: async ({ canvasElement }) => {
    await expectInsideCard(canvasElement, '.sponsor-title');
    await expectInsideCard(canvasElement, '.sponsor-text');
    await expectNotClipped(canvasElement, '.sponsor-title');
  },
};
