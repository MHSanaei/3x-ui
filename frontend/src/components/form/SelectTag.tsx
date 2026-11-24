import { useContext } from 'react';
import type { KeyboardEvent, MouseEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { ConfigProvider } from 'antd';
import type { SelectProps } from 'antd';
import { CloseOutlined } from '@ant-design/icons';
import './SelectTag.css';

type SelectTagProps = Parameters<NonNullable<SelectProps['tagRender']>>[0];

const REMOVE_CLASS = 'select-tag-remove';

// Stops rc-select from opening the dropdown or eating Enter/Space before the click fires.
function keepFromSelect(e: KeyboardEvent | MouseEvent) {
  if ('key' in e && e.key === 'Tab') return;
  e.stopPropagation();
  if (e.type === 'mousedown') e.preventDefault();
}

function SelectTag({ label, value, closable, onClose }: SelectTagProps) {
  const { t } = useTranslation();
  const { getPrefixCls } = useContext(ConfigProvider.ConfigContext);
  const selectCls = getPrefixCls('select');
  const itemCls = `${selectCls}-selection-item`;
  const text =
    typeof label === 'string' || typeof label === 'number' ? String(label) : String(value);

  const remove = (e: MouseEvent<HTMLButtonElement>) => {
    const button = e.currentTarget;
    const root = button.closest(`.${selectCls}`);
    const hadFocus = document.activeElement === button;
    const index = root ? [...root.querySelectorAll(`.${REMOVE_CLASS}`)].indexOf(button) : -1;
    onClose(e);
    if (!root || !hadFocus) return;
    // Keyboard users would otherwise land on <body> once this tag unmounts.
    requestAnimationFrame(() => {
      const rest = root.querySelectorAll<HTMLElement>(`.${REMOVE_CLASS}`);
      const next = rest[Math.min(index, rest.length - 1)] ?? root.querySelector('input');
      next?.focus();
    });
  };

  return (
    <span className={itemCls} title={text}>
      <span className={`${itemCls}-content`}>{label}</span>
      {closable && (
        <button
          type="button"
          className={`${itemCls}-remove ${REMOVE_CLASS}`}
          aria-label={t('removeItem', { item: text })}
          onMouseDown={keepFromSelect}
          onKeyDown={keepFromSelect}
          onClick={remove}
        >
          <CloseOutlined aria-hidden />
        </button>
      )}
    </span>
  );
}

// Ant Design renders each tag's "x" as an aria-hidden, unfocusable span, so screen-reader
// and keyboard users cannot remove a tag; this swaps in a real, named button.
export function selectTagRender(props: SelectTagProps) {
  return <SelectTag {...props} />;
}
