import { describe, expect, it } from 'vitest';
import { Form } from 'antd';
import { fireEvent } from '@testing-library/react';
import { FormProvider, useForm } from 'react-hook-form';

import RealityWatchSection from '@/pages/inbounds/form/security/RealityWatchSection';
import type { RealityWatchEntry } from '@/generated/types';
import { renderWithProviders } from './test-utils';

function Harness({ onDraftChange }: { onDraftChange: (d: RealityWatchEntry) => void }) {
  const methods = useForm({
    defaultValues: { streamSettings: { realitySettings: { target: 'a.example.com:443' } } },
  });
  return (
    <FormProvider {...methods}>
      <Form>
        <RealityWatchSection
          inboundId={null}
          onNode={false}
          scanRealityCandidates={async () => []}
          onDraftChange={onDraftChange}
        />
      </Form>
    </FormProvider>
  );
}

// A new inbound used to show only "save first", so spares meant a create-then-edit round trip.
describe('REALITY watch on a new inbound', () => {
  it('lets the admin set the auto switch and hands it to the form to save with the inbound', () => {
    const drafts: RealityWatchEntry[] = [];
    const { getByRole, getByText } = renderWithProviders(
      <Harness onDraftChange={(d) => drafts.push(d)} />,
    );
    expect(getByText('Find spares')).toBeTruthy();
    fireEvent.click(getByRole('switch'));
    expect(drafts.at(-1)).toEqual({ auto: true, spares: [] });
  });
});
