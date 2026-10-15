import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { Status } from '@/models/status';
import VersionModal from '@/pages/index/VersionModal';
import { HttpUtil, Msg } from '@/utils';

describe('Xray version selection', () => {
  it('checks latest instead of the numeric image version when the container follows its image', async () => {
    vi.mocked(HttpUtil.get).mockResolvedValue(new Msg(true, '', ['latest', 'v26.10.10']));
    const data = { xray: { version: '26.10.10', selectedVersion: 'latest' } };
    render(<VersionModal open status={new Status(data)} onClose={vi.fn()} onBusy={vi.fn()} />);

    const radios = (await screen.findAllByRole('radio')) as HTMLInputElement[];
    expect(radios.map((radio) => radio.checked)).toEqual([true, false]);
  });

  it('keeps the pin selected during fallback and can return to latest', async () => {
    vi.mocked(HttpUtil.get).mockResolvedValue(new Msg(true, '', ['latest', 'v26.10.10']));
    const post = vi.mocked(HttpUtil.post).mockResolvedValue(new Msg(true, ''));
    const data = { xray: { version: '26.10.11', selectedVersion: 'v26.10.10' } };
    render(<VersionModal open status={new Status(data)} onClose={vi.fn()} onBusy={vi.fn()} />);

    const radios = (await screen.findAllByRole('radio')) as HTMLInputElement[];
    expect(radios.map((radio) => radio.checked)).toEqual([false, true]);
    fireEvent.click(radios[0]);
    fireEvent.click(await screen.findByRole('button', { name: 'Confirm' }));
    await waitFor(() => expect(post).toHaveBeenCalledWith('/panel/api/server/installXray/latest'));
  });
});
