import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Checkbox,
  Col,
  DatePicker,
  Drawer,
  Form,
  InputNumber,
  Radio,
  Row,
  Select,
  Space,
  Typography,
} from 'antd';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';

import type { InboundOption } from '@/hooks/useClients';
import { emptyFilters, type ClientFilters } from './filters';

interface FilterDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  filters: ClientFilters;
  onChange: (next: ClientFilters) => void;
  inbounds: InboundOption[];
  protocols: string[];
  groups: string[];
}

const BUCKET_KEYS = ['active', 'expiring', 'depleted', 'deactive', 'online'] as const;

export default function FilterDrawer({
  open,
  onOpenChange,
  filters,
  onChange,
  inbounds,
  protocols,
  groups,
}: FilterDrawerProps) {
  const { t } = useTranslation();

  function patch<K extends keyof ClientFilters>(key: K, value: ClientFilters[K]) {
    onChange({ ...filters, [key]: value });
  }

  const inboundOptions = useMemo(
    () => inbounds.map((ib) => ({
      value: ib.id,
      label: ib.remark?.trim() || ib.tag || '',
    })),
    [inbounds],
  );

  const protocolOptions = useMemo(
    () => protocols.map((p) => ({ value: p, label: p })),
    [protocols],
  );

  const groupOptions = useMemo(
    () => groups.map((g) => ({ value: g, label: g })),
    [groups],
  );

  const dateRange: [Dayjs | null, Dayjs | null] = [
    filters.expiryFrom ? dayjs(filters.expiryFrom) : null,
    filters.expiryTo ? dayjs(filters.expiryTo) : null,
  ];

  return (
    <Drawer
      title={t('pages.clients.filterTitle')}
      open={open}
      onClose={() => onOpenChange(false)}
      width={420}
      destroyOnHidden
      footer={
        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
          <Button onClick={() => onChange(emptyFilters())} danger>
            {t('pages.clients.clearAllFilters')}
          </Button>
          <Button type="primary" onClick={() => onOpenChange(false)}>
            {t('done')}
          </Button>
        </div>
      }
    >
      <Form layout="vertical">
        <Form.Item label={<Typography.Text strong>{t('status')}</Typography.Text>}>
          <Checkbox.Group
            value={filters.buckets}
            onChange={(v) => patch('buckets', v as string[])}
          >
            <Space orientation="vertical">
              {BUCKET_KEYS.map((k) => (
                <Checkbox key={k} value={k}>
                  {bucketLabel(k, t)}
                </Checkbox>
              ))}
            </Space>
          </Checkbox.Group>
        </Form.Item>

        <Form.Item label={t('pages.inbounds.protocol')}>
          <Select
            mode="multiple"
            value={filters.protocols}
            onChange={(v) => patch('protocols', v as string[])}
            options={protocolOptions}
            placeholder={t('pages.inbounds.protocol')}
            maxTagCount="responsive"
            allowClear
          />
        </Form.Item>

        <Form.Item label={t('inbounds')}>
          <Select
            mode="multiple"
            value={filters.inboundIds}
            onChange={(v) => patch('inboundIds', v as number[])}
            options={inboundOptions}
            placeholder={t('inbounds')}
            maxTagCount="responsive"
            allowClear
            showSearch
            optionFilterProp="label"
            listHeight={220}
          />
        </Form.Item>

        <Form.Item label={t('pages.clients.group')}>
          <Select
            mode="multiple"
            value={filters.groups}
            onChange={(v) => patch('groups', v as string[])}
            options={groupOptions}
            placeholder={t('pages.clients.groupPlaceholder')}
            maxTagCount="responsive"
            allowClear
            showSearch
            optionFilterProp="label"
            listHeight={220}
          />
        </Form.Item>

        <Form.Item label={t('pages.clients.expiryTime')}>
          <DatePicker.RangePicker
            value={dateRange}
            onChange={(range) => {
              const from = range?.[0]?.startOf('day').valueOf();
              const to = range?.[1]?.endOf('day').valueOf();
              onChange({ ...filters, expiryFrom: from || undefined, expiryTo: to || undefined });
            }}
            style={{ width: '100%' }}
            allowEmpty={[true, true]}
          />
        </Form.Item>

        <Form.Item label={`${t('pages.clients.traffic')} (GB)`}>
          <Row gutter={8}>
            <Col span={12}>
              <InputNumber
                value={filters.usageFromGB}
                min={0}
                step={1}
                placeholder={t('from')}
                style={{ width: '100%' }}
                onChange={(v) => patch('usageFromGB', typeof v === 'number' ? v : undefined)}
              />
            </Col>
            <Col span={12}>
              <InputNumber
                value={filters.usageToGB}
                min={0}
                step={1}
                placeholder={t('to')}
                style={{ width: '100%' }}
                onChange={(v) => patch('usageToGB', typeof v === 'number' ? v : undefined)}
              />
            </Col>
          </Row>
        </Form.Item>

        <Form.Item label={t('pages.clients.renew')}>
          <Radio.Group
            value={filters.autoRenew}
            onChange={(e) => patch('autoRenew', e.target.value)}
            optionType="button"
            buttonStyle="solid"
            options={[
              { value: '', label: t('all') },
              { value: 'on', label: t('enabled') },
              { value: 'off', label: t('disabled') },
            ]}
          />
        </Form.Item>

        <Form.Item label={t('pages.clients.telegramId')}>
          <Radio.Group
            value={filters.hasTgId}
            onChange={(e) => patch('hasTgId', e.target.value)}
            optionType="button"
            buttonStyle="solid"
            options={[
              { value: '', label: t('all') },
              { value: 'yes', label: t('pages.clients.has') },
              { value: 'no', label: t('pages.clients.hasNot') },
            ]}
          />
        </Form.Item>

        <Form.Item label={t('pages.clients.comment')}>
          <Radio.Group
            value={filters.hasComment}
            onChange={(e) => patch('hasComment', e.target.value)}
            optionType="button"
            buttonStyle="solid"
            options={[
              { value: '', label: t('all') },
              { value: 'yes', label: t('pages.clients.has') },
              { value: 'no', label: t('pages.clients.hasNot') },
            ]}
          />
        </Form.Item>
      </Form>
    </Drawer>
  );
}

function bucketLabel(key: string, t: (k: string) => string): string {
  switch (key) {
    case 'active': return t('subscription.active');
    case 'expiring': return t('depletingSoon');
    case 'depleted': return t('depleted');
    case 'deactive': return t('disabled');
    case 'online': return t('online');
    default: return key;
  }
}
