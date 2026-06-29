import { cloneElement, Fragment, isValidElement, useId, type ReactElement, type ReactNode } from 'react';
import { Col, Row } from 'antd';
import './SettingListItem.css';

interface SettingListItemProps {
  paddings?: 'small' | 'default';
  title?: ReactNode;
  description?: ReactNode;
  children?: ReactNode;
  control?: ReactNode;
}

export default function SettingListItem({
  paddings = 'default',
  title,
  description,
  children,
  control,
}: SettingListItemProps) {
  const padding = paddings === 'small' ? '10px 20px' : '20px';
  const titleId = useId();
  const node = control ?? children;
  const labelledNode = title && isValidElement(node) && node.type !== Fragment
    ? cloneElement(node as ReactElement<{ 'aria-labelledby'?: string }>, { 'aria-labelledby': titleId })
    : node;
  return (
    <div className="setting-list-item" style={{ padding }}>
      <Row gutter={[8, 16]} style={{ width: '100%' }}>
        <Col xs={24} lg={12}>
          <div className="setting-list-meta">
            {title && <div className="setting-list-title" id={titleId}>{title}</div>}
            {description && <div className="setting-list-description">{description}</div>}
          </div>
        </Col>
        <Col xs={24} lg={12}>
          {labelledNode}
        </Col>
      </Row>
    </div>
  );
}
