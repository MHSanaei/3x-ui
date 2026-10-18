import { useState } from 'react';

import { useSponsorsQuery } from '@/api/queries/useSponsorsQuery';
import {
  dismissSponsor,
  isSponsorDismissed,
  sponsorsForSlot,
  type SponsorSlot as Slot,
} from '@/lib/sponsors';
import SponsorCard, { type SponsorCardVariant } from './SponsorCard';

interface SponsorSlotProps {
  slot: Slot;
  variant?: SponsorCardVariant;
  className?: string;
}

export default function SponsorSlot({ slot, variant = 'banner', className }: SponsorSlotProps) {
  const { data } = useSponsorsQuery();
  const [, setDismissTick] = useState(0);

  // Re-filtered each render so a close (dismissTick bump) re-reads localStorage.
  const visible = sponsorsForSlot(data.sponsors, slot).filter(
    (s) => !isSponsorDismissed(s.id, slot),
  );

  if (visible.length === 0) return null;
  const sponsor = visible[0];

  return (
    <div className={className}>
      <SponsorCard
        sponsor={sponsor}
        variant={variant}
        onClose={() => {
          dismissSponsor(sponsor.id, slot);
          setDismissTick((n) => n + 1);
        }}
      />
    </div>
  );
}
