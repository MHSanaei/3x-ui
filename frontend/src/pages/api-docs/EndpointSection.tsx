import type { ComponentType } from 'react';
import { Table } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { DownOutlined, RightOutlined } from '@ant-design/icons';
import EndpointRow from './EndpointRow';
import type { Endpoint } from './EndpointRow';
import { safeInlineHtml } from './endpoints.js';
import './EndpointSection.css';

interface SubHeader {
  name: string;
  desc?: string;
}

export interface Section {
  id: string;
  title: string;
  description?: string;
  endpoints: Endpoint[];
  subHeader?: SubHeader[];
}

interface EndpointSectionProps {
  section: Section;
  icon?: ComponentType<{ className?: string }> | null;
  collapsed?: boolean;
  onToggle?: () => void;
}

const subHeaderColumns: ColumnsType<SubHeader> = [
  { title: 'Header', dataIndex: 'name', key: 'name', width: 240 },
  {
    title: 'Description',
    dataIndex: 'desc',
    key: 'desc',
    render: (value: string) => (
      <span dangerouslySetInnerHTML={{ __html: safeInlineHtml(value || '') }} />
    ),
  },
];

export default function EndpointSection({
  section,
  icon: Icon = null,
  collapsed = false,
  onToggle,
}: EndpointSectionProps) {
  const endpointLabel = section.endpoints.length === 1
    ? '1 endpoint'
    : `${section.endpoints.length} endpoints`;

  return (
    <section id={section.id} className="api-section">
      <div className="section-header" onClick={onToggle}>
        <div className="section-header-left">
          {collapsed ? <RightOutlined className="collapse-icon" /> : <DownOutlined className="collapse-icon" />}
          {Icon && <Icon className="section-icon" />}
          <h2 className="section-title">{section.title}</h2>
        </div>
        <span className="endpoint-count">{endpointLabel}</span>
      </div>

      {section.description && !collapsed && (
        <p
          className="section-description"
          dangerouslySetInnerHTML={{ __html: safeInlineHtml(section.description) }}
        />
      )}

      {section.subHeader && !collapsed && (
        <div className="sub-header-block">
          <div className="section-block-label">Response headers</div>
          <Table
            columns={subHeaderColumns}
            dataSource={section.subHeader}
            pagination={false}
            size="small"
            rowKey="name"
          />
        </div>
      )}

      <div className="endpoints" style={{ display: collapsed ? 'none' : undefined }}>
        {section.endpoints.map((endpoint, idx) => (
          <EndpointRow key={idx} endpoint={endpoint} />
        ))}
      </div>
    </section>
  );
}
