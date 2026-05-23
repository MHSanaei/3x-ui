import type { CSSProperties, ReactNode } from 'react';
import './InputAddon.css';

interface InputAddonProps {
  children: ReactNode;
  className?: string;
  style?: CSSProperties;
  onClick?: () => void;
}

export default function InputAddon({ children, className = '', style, onClick }: InputAddonProps) {
  return (
    <span
      className={`input-addon ${className}`.trim()}
      style={style}
      onClick={onClick}
    >
      {children}
    </span>
  );
}
