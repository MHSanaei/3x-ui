import type { Meta, StoryObj } from '@storybook/react-vite';

import { ClientSpeedTag } from './ClientSpeedTag';

const meta = {
  title: 'Clients/ClientSpeedTag',
  component: ClientSpeedTag,
  tags: ['autodocs'],
} satisfies Meta<typeof ClientSpeedTag>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Active: Story = {
  args: { speed: { up: 1_450_000, down: 8_900_000 } },
};

export const Idle: Story = {
  args: { speed: { up: 0, down: 0 } },
};
