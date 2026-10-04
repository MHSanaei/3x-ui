import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import PostQuantumQrNotice from './PostQuantumQrNotice';

const meta = {
  title: 'Inbounds/PostQuantumQrNotice',
  component: PostQuantumQrNotice,
  tags: ['autodocs'],
  argTypes: {
    size: { description: 'Matches the adjacent QR and copy action buttons.' },
  },
} satisfies Meta<typeof PostQuantumQrNotice>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Explanation: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole('button', { name: 'QR code unavailable' }));
    await expect(
      await within(document.body).findByText(/Copy the link and import it from the clipboard/),
    ).toBeVisible();
  },
};
