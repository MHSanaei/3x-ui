import { useTranslation } from 'react-i18next';
import { Button, Card, Col, Empty, Input, InputNumber, Row, Select, Space } from 'antd';
import { ArrowDownOutlined, ArrowUpOutlined, DeleteOutlined, PlusOutlined } from '@ant-design/icons';

import type { FallbackRow } from '@/schemas/forms/inbound-form';

interface FallbacksCardProps {
  fallbacks: FallbackRow[];
  fallbackChildOptions: { label: string; value: number }[];
  addFallback: () => void;
  updateFallback: (rowKey: string, patch: Partial<FallbackRow>) => void;
  removeFallback: (idx: number) => void;
  moveFallback: (idx: number, direction: -1 | 1) => void;
  addAllFallbacks: () => void;
}

export default function FallbacksCard({
  fallbacks,
  fallbackChildOptions,
  addFallback,
  updateFallback,
  removeFallback,
  moveFallback,
  addAllFallbacks,
}: FallbacksCardProps) {
  const { t } = useTranslation();

  const addButtons = (
    <Space size={8} wrap>
      <Button type="primary" ghost size="small" icon={<PlusOutlined />} onClick={addFallback}>
        {t('pages.inbounds.fallbacks.add') || 'Add fallback'}
      </Button>
      <Button
        size="small"
        onClick={addAllFallbacks}
        disabled={fallbackChildOptions.length === 0 || fallbacks.length >= fallbackChildOptions.length}
        title={t('pages.inbounds.form.addAllFallbackTooltip')}
      >
        {t('pages.inbounds.form.addAll')}
      </Button>
    </Space>
  );

  return (
    <Card
      size="small"
      className="mt-12"
      title={t('pages.inbounds.fallbacks.title') || 'Fallbacks'}
      extra={addButtons}
    >
      {fallbacks.length === 0 ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          styles={{ image: { height: 36 } }}
          description={t('pages.inbounds.fallbacks.empty') || 'No fallbacks yet'}
          style={{ margin: '4px 0 12px' }}
        />
      ) : (
        fallbacks.map((record, idx) => (
          <Card
            key={record.rowKey}
            type="inner"
            size="small"
            style={{ marginBottom: 8 }}
            styles={{ body: { padding: 12 } }}
          >
            <Space.Compact block style={{ marginBottom: 8 }}>
              <Select
                aria-label={t('pages.inbounds.fallbacks.pickInbound')}
                value={record.childId}
                options={fallbackChildOptions}
                placeholder={t('pages.inbounds.fallbacks.pickInbound') || 'Pick an inbound'}
                allowClear
                showSearch={{
                  filterOption: (input, option) =>
                    ((option?.label as string) || '').toLowerCase().includes(input.toLowerCase()),
                }}
                style={{ width: '100%' }}
                onChange={(v) => updateFallback(record.rowKey, { childId: v ?? null })}
              />
              <Button
                aria-label={t('pages.inbounds.form.moveUp')}
                disabled={idx === 0}
                onClick={() => moveFallback(idx, -1)}
                title={t('pages.inbounds.form.moveUp')}
                icon={<ArrowUpOutlined />}
              />
              <Button
                aria-label={t('pages.inbounds.form.moveDown')}
                disabled={idx === fallbacks.length - 1}
                onClick={() => moveFallback(idx, 1)}
                title={t('pages.inbounds.form.moveDown')}
                icon={<ArrowDownOutlined />}
              />
              <Button aria-label={t('delete')} danger onClick={() => removeFallback(idx)} icon={<DeleteOutlined />} />
            </Space.Compact>
            <Row gutter={[8, 8]}>
              <Col xs={24} sm={12}>
                <Input
                  prefix="SNI"
                  placeholder={t('pages.inbounds.fallbacks.matchAny') || 'any'}
                  value={record.name}
                  onChange={(e) => updateFallback(record.rowKey, { name: e.target.value })}
                />
              </Col>
              <Col xs={24} sm={12}>
                <Input
                  prefix="ALPN"
                  placeholder={t('pages.inbounds.fallbacks.matchAny') || 'any'}
                  value={record.alpn}
                  onChange={(e) => updateFallback(record.rowKey, { alpn: e.target.value })}
                />
              </Col>
              <Col xs={24} sm={12}>
                <Input
                  prefix="Path"
                  placeholder="/"
                  value={record.path}
                  onChange={(e) => updateFallback(record.rowKey, { path: e.target.value })}
                />
              </Col>
              <Col xs={24} sm={12}>
                <Input
                  prefix="Dest"
                  placeholder={t('pages.inbounds.fallbacks.destPlaceholder') || 'auto'}
                  value={record.dest}
                  onChange={(e) => updateFallback(record.rowKey, { dest: e.target.value })}
                />
              </Col>
              <Col xs={24} sm={12}>
                <InputNumber
                  prefix="xver"
                  min={0}
                  max={2}
                  style={{ width: '100%' }}
                  value={record.xver}
                  onChange={(v) => updateFallback(record.rowKey, { xver: Number(v) || 0 })}
                />
              </Col>
            </Row>
          </Card>
        ))
      )}
    </Card>
  );
}
