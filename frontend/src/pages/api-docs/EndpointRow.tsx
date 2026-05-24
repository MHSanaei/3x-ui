import { Table, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { methodColors, safeInlineHtml } from './endpoints.js';
import CodeBlock from './CodeBlock';
import './EndpointRow.css';

interface Param {
  name: string;
  in?: string;
  type?: string;
  desc?: string;
}

export interface Endpoint {
  method: string;
  path: string;
  summary?: string;
  params?: Param[];
  body?: string;
  response?: string;
  errorResponse?: string;
}

const paramColumns: ColumnsType<Param> = [
  { title: 'Name', dataIndex: 'name', key: 'name', width: 180 },
  { title: 'In', dataIndex: 'in', key: 'in', width: 100 },
  { title: 'Type', dataIndex: 'type', key: 'type', width: 120 },
  { title: 'Description', dataIndex: 'desc', key: 'desc' },
];

export default function EndpointRow({ endpoint }: { endpoint: Endpoint }) {
  const tagColor = (methodColors as Record<string, string>)[endpoint.method] || 'default';
  const hasParams = Array.isArray(endpoint.params) && endpoint.params.length > 0;

  return (
    <div className="endpoint-row">
      <div className="endpoint-header">
        <Tag color={tagColor} className="method-tag">{endpoint.method}</Tag>
        <code className="endpoint-path">{endpoint.path}</code>
      </div>

      {endpoint.summary && (
        <p
          className="endpoint-summary"
          dangerouslySetInnerHTML={{ __html: safeInlineHtml(endpoint.summary) }}
        />
      )}

      {hasParams && (
        <div className="endpoint-block">
          <div className="block-label">Parameters</div>
          <Table
            columns={paramColumns}
            dataSource={endpoint.params}
            pagination={false}
            size="small"
            rowKey="name"
          />
        </div>
      )}

      {endpoint.body && (
        <div className="endpoint-block">
          <div className="block-label">Request body</div>
          <CodeBlock code={endpoint.body} lang="json" />
        </div>
      )}

      {endpoint.response && (
        <div className="endpoint-block">
          <div className="block-label">Response</div>
          <CodeBlock code={endpoint.response} lang="json" />
        </div>
      )}

      {endpoint.errorResponse && (
        <div className="endpoint-block">
          <div className="block-label error-label">Error response</div>
          <CodeBlock code={endpoint.errorResponse} lang="json" />
        </div>
      )}
    </div>
  );
}
