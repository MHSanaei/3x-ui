import { Form } from 'antd';
import { FormProvider, useForm } from 'react-hook-form';
import { describe, expect, it } from 'vitest';
import { fireEvent, waitFor } from '@testing-library/react';

import WireguardFields from '@/pages/inbounds/form/protocols/wireguard';
import { renderWithProviders } from './test-utils';

function Harness() {
  const form = useForm({ defaultValues: { settings: { secretKey: 'private-key' } } });
  return (
    <FormProvider {...form}>
      <Form>
        <WireguardFields wgPubKey="public-key" regenInboundWg={() => {}} />
      </Form>
    </FormProvider>
  );
}

describe('WireGuard VK TURN form', () => {
  it('initializes the proxy port on enable and preserves an edited port across toggles', async () => {
    const { getByRole } = renderWithProviders(<Harness />);
    const toggle = getByRole('switch', { name: 'Tunnel through VK' });
    fireEvent.click(toggle);
    const port = getByRole('spinbutton', { name: 'VK proxy UDP port' }) as HTMLInputElement;
    await waitFor(() => expect(port.value).toBe('56000'));
    fireEvent.change(port, { target: { value: '56100' } });
    fireEvent.blur(port);
    fireEvent.click(toggle);
    fireEvent.click(toggle);
    await waitFor(() =>
      expect(
        (getByRole('spinbutton', { name: 'VK proxy UDP port' }) as HTMLInputElement).value,
      ).toBe('56100'),
    );
  });
});
