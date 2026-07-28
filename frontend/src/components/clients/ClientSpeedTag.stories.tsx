import type { Meta, StoryObj } from '@storybook/react-vite';

import { ClientSpeedTag } from './ClientSpeedTag';

const meta = {
  title: 'Clients/ClientSpeedTag',
  component: ClientSpeedTag,
  tags: ['autodocs'],
  parameters: {
    docs: {
      description: {
        component:
          'Blue tag showing a live upload/download speed for one client, formatted for readability. Shown next to online clients that have active traffic.',
      },
    },
  },
  argTypes: {
    speed: { description: 'Live upload/download rate in bytes per second (`{ up, down }`).' },
  },
} satisfies Meta<typeof ClientSpeedTag>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Active: Story = {
  args: { speed: { up: 1_450_000, down: 8_900_000 } },
};

export const Idle: Story = {
  args: { speed: { up: 0, down: 0 } },
};
