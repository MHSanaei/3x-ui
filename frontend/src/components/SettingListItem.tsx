import type { ReactNode } from 'react';
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
  return (
    <div className="setting-list-item" style={{ padding }}>
      <Row gutter={[8, 16]} style={{ width: '100%' }}>
        <Col xs={24} lg={12}>
          <div className="setting-list-meta">
            {title && <div className="setting-list-title">{title}</div>}
            {description && <div className="setting-list-description">{description}</div>}
          </div>
        </Col>
        <Col xs={24} lg={12}>
          {control ?? children}
        </Col>
      </Row>
    </div>
  );
}
