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
    // oxlint cannot see through the conditional role/tabIndex/onKeyDown below,
    // which is exactly what makes the clickable variant accessible.
    // oxlint-disable-next-line jsx-a11y/no-static-element-interactions
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
