import { useRef, useState } from 'react';
import { Input, Typography } from 'antd';

import { HttpUtil } from '@/utils';

interface GoRegexInputProps {
  value: string;
  ariaLabel?: string;
  placeholder?: string;
  maxLength?: number;
  onChange: (value: string) => void;
  externalError?: string;
}

export async function validateGoRegex(value: string): Promise<string> {
  const result = await HttpUtil.post(
    '/panel/api/setting/validateRegex',
    { regex: value },
    { silent: true },
  );
  return result.success ? '' : result.msg || 'Invalid Go RE2 regular expression';
}

export default function GoRegexInput({
  value,
  ariaLabel,
  placeholder,
  maxLength = 2048,
  onChange,
  externalError,
}: GoRegexInputProps) {
  const [error, setError] = useState('');
  const validationSequence = useRef(0);

  async function validate() {
    const sequence = ++validationSequence.current;
    const nextError = await validateGoRegex(value);
    if (sequence === validationSequence.current) {
      setError(nextError);
    }
  }

  const displayError = externalError ?? error;

  return (
    <div style={{ width: '100%' }}>
      <Input
        value={value}
        aria-label={ariaLabel}
        placeholder={placeholder}
        maxLength={maxLength}
        status={displayError ? 'error' : undefined}
        onChange={(event) => {
          validationSequence.current += 1;
          setError('');
          onChange(event.target.value);
        }}
        onBlur={() => void validate()}
      />
      {displayError && <Typography.Text type="danger">{displayError}</Typography.Text>}
    </div>
  );
}
