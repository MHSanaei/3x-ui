import { useEffect, useState } from 'react';

import { useSponsorsQuery } from '@/api/queries/useSponsorsQuery';
import {
  dismissSponsor,
  isSponsorDismissed,
  sponsorsForSlot,
  type SponsorSlot as Slot,
} from '@/lib/sponsors';
import SponsorCard, { type SponsorCardVariant } from './SponsorCard';

const ROTATE_MS = 30_000;

interface SponsorSlotProps {
  slot: Slot;
  variant?: SponsorCardVariant;
  iconOnly?: boolean;
  rotate?: boolean;
  className?: string;
}

export default function SponsorSlot({
  slot,
  variant = 'banner',
  iconOnly,
  rotate,
  className,
}: SponsorSlotProps) {
  const { data } = useSponsorsQuery();
  const [, setDismissTick] = useState(0);
  const [index, setIndex] = useState(0);

  // Re-filtered each render so a close (dismissTick bump) re-reads localStorage.
  const visible = sponsorsForSlot(data.sponsors, slot).filter(
    (s) => !isSponsorDismissed(s.id, slot),
  );

  useEffect(() => {
    if (!rotate || visible.length < 2) return;
    const timer = window.setInterval(() => setIndex((i) => i + 1), ROTATE_MS);
    return () => window.clearInterval(timer);
  }, [rotate, visible.length]);

  if (visible.length === 0) return null;
  const sponsor = visible[(rotate ? index : 0) % visible.length];

  return (
    <div className={className}>
      <SponsorCard
        sponsor={sponsor}
        variant={variant}
        iconOnly={iconOnly}
        onClose={() => {
          dismissSponsor(sponsor.id, slot);
          setDismissTick((n) => n + 1);
        }}
      />
    </div>
  );
}
