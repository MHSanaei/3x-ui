import { fireEvent, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import PanelUpdateModal from '@/pages/index/PanelUpdateModal';
import { HttpUtil } from '@/utils';
import { renderWithProviders } from './test-utils';

describe('offline update modal', () => {
  afterEach(() => vi.mocked(HttpUtil.post).mockReset());

  it('uploads the selected ZIP as a binary request', async () => {
    const onBusy = vi.fn();
    vi.mocked(HttpUtil.post).mockReturnValue(new Promise(() => {}));
    renderWithProviders(<PanelUpdateModal open onClose={vi.fn()} onBusy={onBusy} />);

    const select = screen.getByLabelText('Select update ZIP');
    const apply = screen.getByRole('button', { name: 'Upload and apply' });
    expect((apply as HTMLButtonElement).disabled).toBe(true);
    const file = new File(['example'], 'update.zip', { type: 'application/zip' });
    fireEvent.change(select, { target: { files: [file] } });
    fireEvent.click(apply);

    await waitFor(() =>
      expect(HttpUtil.post).toHaveBeenCalledWith(
        '/panel/api/server/uploadUpdate',
        file,
        expect.objectContaining({ headers: { 'Content-Type': 'application/zip' } }),
      ),
    );
    expect(onBusy).toHaveBeenCalledWith(expect.objectContaining({ busy: true }));
  });
});
