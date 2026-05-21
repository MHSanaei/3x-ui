import type { ReactNode } from 'react';
import { Col, List, Row } from 'antd';

interface SettingListItemProps {
  paddings?: 'small' | 'default';
  title?: ReactNode;
  description?: ReactNode;
  children?: ReactNode;
}

export default function SettingListItem({
  paddings = 'default',
  title,
  description,
  children,
}: SettingListItemProps) {
  const padding = paddings === 'small' ? '10px 20px' : '20px';
  return (
    <List.Item style={{ padding }}>
      <Row gutter={[8, 16]} style={{ width: '100%' }}>
        <Col xs={24} lg={12}>
          <List.Item.Meta title={title} description={description} />
        </Col>
        <Col xs={24} lg={12}>
          {children}
        </Col>
      </Row>
    </List.Item>
  );
}
