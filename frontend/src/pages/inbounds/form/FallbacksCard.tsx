import { useTranslation } from 'react-i18next';
import { Button, Card, Empty, Input, InputNumber, Select, Space } from 'antd';
import { ArrowDownOutlined, ArrowUpOutlined, DeleteOutlined, PlusOutlined } from '@ant-design/icons';

import { InputAddon } from '@/components/ui';
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
  return (
    <Card size="small" className="mt-12" title={t('pages.inbounds.fallbacks.title') || 'Fallbacks'}>
      {fallbacks.length === 0 && (
        <Empty
          description={t('pages.inbounds.fallbacks.empty') || 'No fallbacks yet'}
          styles={{ image: { height: 40 } }}
          style={{ margin: '8px 0 12px' }}
        />
      )}
      {fallbacks.map((record, idx) => (
        <div
          key={record.rowKey}
          style={{ border: '1px solid var(--app-border-tertiary)', borderRadius: 6, padding: '10px 12px', marginBottom: 8 }}
        >
          <Space.Compact block style={{ marginBottom: 6 }}>
            <Select
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
              disabled={idx === 0}
              onClick={() => moveFallback(idx, -1)}
              title={t('pages.inbounds.form.moveUp')}
            >
              <ArrowUpOutlined />
            </Button>
            <Button
              disabled={idx === fallbacks.length - 1}
              onClick={() => moveFallback(idx, 1)}
              title={t('pages.inbounds.form.moveDown')}
            >
              <ArrowDownOutlined />
            </Button>
            <Button danger onClick={() => removeFallback(idx)}>
              <DeleteOutlined />
            </Button>
          </Space.Compact>
          <Space.Compact block>
            <InputAddon>SNI</InputAddon>
            <Input
              placeholder={t('pages.inbounds.fallbacks.matchAny') || 'any'}
              value={record.name}
              onChange={(e) => updateFallback(record.rowKey, { name: e.target.value })}
            />
            <InputAddon>ALPN</InputAddon>
            <Input
              placeholder={t('pages.inbounds.fallbacks.matchAny') || 'any'}
              value={record.alpn}
              onChange={(e) => updateFallback(record.rowKey, { alpn: e.target.value })}
            />
            <InputAddon>Path</InputAddon>
            <Input
              placeholder="/"
              value={record.path}
              onChange={(e) => updateFallback(record.rowKey, { path: e.target.value })}
            />
            <InputAddon>Dest</InputAddon>
            <Input
              placeholder={t('pages.inbounds.fallbacks.destPlaceholder') || 'auto'}
              value={record.dest}
              onChange={(e) => updateFallback(record.rowKey, { dest: e.target.value })}
            />
            <InputAddon>xver</InputAddon>
            <InputNumber
              min={0}
              max={2}
              value={record.xver}
              onChange={(v) => updateFallback(record.rowKey, { xver: Number(v) || 0 })}
            />
          </Space.Compact>
        </div>
      ))}
      <Space>
        <Button size="small" onClick={addFallback}>
          <PlusOutlined /> {t('pages.inbounds.fallbacks.add') || 'Add fallback'}
        </Button>
        <Button
          size="small"
          onClick={addAllFallbacks}
          disabled={fallbackChildOptions.length === 0
            || fallbacks.length >= fallbackChildOptions.length}
          title={t('pages.inbounds.form.addAllFallbackTooltip')}
        >
          {t('pages.inbounds.form.addAll')}
        </Button>
      </Space>
    </Card>
  );
}
