import type { CSSProperties, ReactNode } from 'react';
import { activateOnKey } from '@/utils/a11y';
import './InputAddon.css';

interface InputAddonProps {
  children: ReactNode;
  className?: string;
  style?: CSSProperties;
  onClick?: () => void;
  ariaLabel?: string;
}

export default function InputAddon({ children, className = '', style, onClick, ariaLabel }: InputAddonProps) {
  return (
    <span
      className={`input-addon ${className}`.trim()}
      style={style}
      onClick={onClick}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
      aria-label={onClick ? ariaLabel : undefined}
      onKeyDown={onClick ? activateOnKey(onClick) : undefined}
    >
      {children}
    </span>
  );
}
