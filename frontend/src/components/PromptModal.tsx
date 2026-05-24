import { useEffect, useRef, useState } from 'react';
import { Input, Modal } from 'antd';
import type { InputRef } from 'antd';

interface PromptModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  okText?: string;
  type?: 'input' | 'textarea';
  initialValue?: string;
  loading?: boolean;
  onConfirm: (value: string) => void;
}

export default function PromptModal({
  open,
  onClose,
  title,
  okText = 'OK',
  type = 'input',
  initialValue = '',
  loading = false,
  onConfirm,
}: PromptModalProps) {
  const [value, setValue] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const inputRef = useRef<InputRef | null>(null);

  useEffect(() => {
    if (open) {
      setValue(initialValue);
      setTimeout(() => {
        if (type === 'textarea') textareaRef.current?.focus();
        else inputRef.current?.focus();
      }, 50);
    }
  }, [open, initialValue, type]);

  function onKeydown(e: React.KeyboardEvent<HTMLTextAreaElement | HTMLInputElement>) {
    if (type !== 'textarea' && e.key === 'Enter') {
      e.preventDefault();
      onConfirm(value);
      return;
    }
    if (type === 'textarea' && e.ctrlKey && e.key.toLowerCase() === 's') {
      e.preventDefault();
      onConfirm(value);
    }
  }

  return (
    <Modal
      open={open}
      title={title}
      okText={okText}
      cancelText="Cancel"
      mask={{ closable: false }}
      confirmLoading={loading}
      onOk={() => onConfirm(value)}
      onCancel={onClose}
      destroyOnHidden
    >
      {type === 'textarea' ? (
        <Input.TextArea
          ref={(el) => { textareaRef.current = (el as unknown as { resizableTextArea?: { textArea: HTMLTextAreaElement } })?.resizableTextArea?.textArea ?? null; }}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          autoSize={{ minRows: 10, maxRows: 20 }}
          onKeyDown={onKeydown}
        />
      ) : (
        <Input
          ref={inputRef}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={onKeydown}
        />
      )}
    </Modal>
  );
}
