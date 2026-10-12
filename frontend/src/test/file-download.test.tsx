import { afterEach, describe, expect, it, vi } from 'vitest';

import { FileManager } from '@/utils';

const { createObjectURL, revokeObjectURL } = URL;

afterEach(() => {
  URL.createObjectURL = createObjectURL;
  URL.revokeObjectURL = revokeObjectURL;
  vi.restoreAllMocks();
});

describe('FileManager.downloadTextFile', () => {
  // Android's MediaStore appends the blob type's own extension when the name's
  // extension maps elsewhere, so a text/plain peer.conf lands as peer.conf.txt.
  it('hands the browser an untyped blob so a mobile save keeps the given name', () => {
    const blobs: Blob[] = [];
    URL.createObjectURL = vi.fn((blob: Blob) => {
      blobs.push(blob);
      return 'blob:download';
    });
    URL.revokeObjectURL = vi.fn();
    let savedAs = '';
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (
      this: HTMLAnchorElement,
    ) {
      savedAs = this.download;
    });

    FileManager.downloadTextFile('[Interface]\n', 'alice.conf');

    expect(savedAs).toBe('alice.conf');
    expect(blobs.map((blob) => blob.type)).toEqual(['application/octet-stream']);
  });
});
