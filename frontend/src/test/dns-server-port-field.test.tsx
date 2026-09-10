import { describe, expect, it } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';

import DnsServerModal from '@/pages/xray/dns/DnsServerModal';
import { renderWithProviders } from './test-utils';

describe('DnsServerModal port field', () => {
  it('hides the port for an encrypted address, whose port lives in the URL', () => {
    renderWithProviders(
      <DnsServerModal
        open
        server="https://dns.example.com/dns-query"
        isEdit
        onClose={() => {}}
        onConfirm={() => {}}
      />,
    );

    expect(screen.queryByLabelText('Port')).toBeNull();
  });

  it('offers the port for a plain address and for DoT', () => {
    renderWithProviders(
      <DnsServerModal
        open
        server="tls://dns.example.com"
        isEdit
        onClose={() => {}}
        onConfirm={() => {}}
      />,
    );

    expect(screen.getByLabelText('Port')).toBeTruthy();
  });

  it('drops the port field as soon as the address becomes a DoH URL', () => {
    renderWithProviders(
      <DnsServerModal open server="1.1.1.1" isEdit onClose={() => {}} onConfirm={() => {}} />,
    );

    expect(screen.getByLabelText('Port')).toBeTruthy();
    fireEvent.change(screen.getByLabelText('Address'), {
      target: { value: 'https://dns.example.com/dns-query' },
    });
    expect(screen.queryByLabelText('Port')).toBeNull();
  });
});
