import type { DBInbound } from '@/models/dbinbound';
import AttachClientPickerModal from './AttachClientPickerModal';

interface AttachClientsModalProps {
  open: boolean;
  source: DBInbound | null;
  dbInbounds: DBInbound[];
  onClose: () => void;
  onAttached?: () => void;
}

export default function AttachClientsModal({
  open,
  source,
  dbInbounds,
  onClose,
  onAttached,
}: AttachClientsModalProps) {
  if (!open || !source) return null;
  return (
    <AttachClientPickerModal
      key={source.id}
      inbound={source}
      sourceInbounds={dbInbounds}
      onClose={onClose}
      onAttached={onAttached}
    />
  );
}
