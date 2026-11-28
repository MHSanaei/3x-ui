import type { DBInbound } from '@/models/dbinbound';
import AttachClientPickerModal from './AttachClientPickerModal';

interface AttachExistingClientsModalProps {
  open: boolean;
  target: DBInbound | null;
  onClose: () => void;
  onAttached?: () => void;
}

export default function AttachExistingClientsModal({
  open,
  target,
  onClose,
  onAttached,
}: AttachExistingClientsModalProps) {
  if (!open || !target) return null;
  return (
    <AttachClientPickerModal
      key={target.id}
      inbound={target}
      onClose={onClose}
      onAttached={onAttached}
    />
  );
}
