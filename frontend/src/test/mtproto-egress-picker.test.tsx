import type { ReactNode } from 'react';
import { Form } from 'antd';
import { fireEvent, waitFor } from '@testing-library/react';
import { FormProvider, useForm } from 'react-hook-form';
import { afterEach, describe, expect, it, vi } from 'vitest';

import MtprotoFields from '@/pages/inbounds/form/protocols/mtproto';
import { HttpUtil, Msg } from '@/utils';
import { renderWithProviders } from './test-utils';

afterEach(() => {
  vi.restoreAllMocks();
});

// The picker exists so Telegram traffic can be sent through a proxy; offering
// the block outbound there looks like a working selection and drops the traffic.
function mockConfigWithBlockOutbound() {
  const payload = {
    xraySetting: {
      outbounds: [
        { tag: 'direct', protocol: 'freedom' },
        { tag: 'blocked', protocol: 'blackhole' },
        { tag: 'warp', protocol: 'wireguard' },
      ],
    },
  };
  vi.spyOn(HttpUtil, 'post').mockResolvedValue(new Msg(true, '', JSON.stringify(payload)));
}

function Harness({ children }: { children: ReactNode }) {
  const methods = useForm({ defaultValues: { settings: { routeThroughXray: true } } });
  return (
    <FormProvider {...methods}>
      <Form>{children}</Form>
    </FormProvider>
  );
}

// The egress picker is the form's only searchable select, so it can be found
// without depending on the field id react-hook-form generates.
function egressPicker(): HTMLElement {
  const select = document.querySelector('.ant-select-show-search');
  if (!select) throw new Error('egress picker not rendered');
  return select as HTMLElement;
}

function egressOptions(): string[] {
  const select = egressPicker();
  fireEvent.mouseDown(select);
  const options = Array.from(
    document.querySelectorAll(
      '.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option',
    ),
  ).map((o) => (o.getAttribute('title') ?? o.textContent ?? '').trim());
  fireEvent.keyDown(select, { key: 'Escape' });
  return options;
}

describe('mtproto egress picker', () => {
  it('offers the routable tags and not the block outbound', async () => {
    mockConfigWithBlockOutbound();
    renderWithProviders(
      <Harness>
        <MtprotoFields />
      </Harness>,
    );

    await waitFor(() => expect(egressOptions()).toContain('direct'));

    const options = egressOptions();
    expect(options).toContain('direct');
    expect(options).toContain('warp');
    expect(options).not.toContain('blocked');
  });
});
