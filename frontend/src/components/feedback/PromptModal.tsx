import { useEffect, useRef, useState } from 'react';
import { Input, Modal } from 'antd';
import type { InputRef } from 'antd';
import { useTranslation } from 'react-i18next';

import JsonEditor from '@/components/form/JsonEditor';

interface PromptModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  okText?: string;
  type?: 'input' | 'textarea';
  initialValue?: string;
  loading?: boolean;
  json?: boolean;
  onConfirm: (value: string) => void;
}

export default function PromptModal({
  open,
  onClose,
  title,
  okText,
  type = 'input',
  initialValue = '',
  loading = false,
  json = false,
  onConfirm,
}: PromptModalProps) {
  const { t } = useTranslation();
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
      okText={okText ?? t('confirm')}
      cancelText={t('cancel')}
      mask={{ closable: false }}
      confirmLoading={loading}
      onOk={() => onConfirm(value)}
      onCancel={onClose}
      destroyOnHidden
    >
      {json ? (
        <JsonEditor value={value} onChange={setValue} minHeight="240px" maxHeight="60vh" />
      ) : type === 'textarea' ? (
        <Input.TextArea
          ref={(el) => { textareaRef.current = (el as unknown as { resizableTextArea?: { textArea: HTMLTextAreaElement } })?.resizableTextArea?.textArea ?? null; }}
          aria-label={title}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          autoSize={{ minRows: 10, maxRows: 20 }}
          onKeyDown={onKeydown}
        />
      ) : (
        <Input
          ref={inputRef}
          aria-label={title}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={onKeydown}
        />
      )}
    </Modal>
  );
}
