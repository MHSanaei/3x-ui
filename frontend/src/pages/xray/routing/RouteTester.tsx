import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Col, Input, InputNumber, Row, Select, Space, Tag } from 'antd';
import { AimOutlined } from '@ant-design/icons';

import { HttpUtil } from '@/utils';
import { useInboundOptions } from '@/api/queries/useInboundOptions';
import { buildRemarkByTag, formatInboundTag } from './helpers';

interface RouteTesterProps {
  inboundTags: string[];
  isMobile: boolean;
}

// Mirror of the /xray/routeTest response (RoutingService.TestRoute).
interface RouteTestResult {
  matched: boolean;
  outboundTag: string;
  groupTags?: string[];
}

const PROTOCOL_OPTIONS = ['http', 'tls', 'quic', 'bittorrent'].map((p) => ({ label: p, value: p }));

export default function RouteTester({ inboundTags, isMobile }: RouteTesterProps) {
  const { t } = useTranslation();
  const { data: inboundOptions } = useInboundOptions();
  const remarkByTag = useMemo(() => buildRemarkByTag(inboundOptions || []), [inboundOptions]);
  const [dest, setDest] = useState('');
  const [port, setPort] = useState<number | null>(443);
  const [network, setNetwork] = useState('tcp');
  const [inboundTag, setInboundTag] = useState<string | undefined>(undefined);
  const [protocol, setProtocol] = useState<string | undefined>(undefined);
  const [testing, setTesting] = useState(false);
  const [result, setResult] = useState<RouteTestResult | null>(null);

  async function run() {
    const value = dest.trim();
    if (!value) return;
    // Domains never contain ':' and a pure dotted-quad is an IPv4 address;
    // everything else is treated as a domain.
    const isIp = /^(\d{1,3}\.){3}\d{1,3}$/.test(value) || value.includes(':');
    setTesting(true);
    setResult(null);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/routeTest', {
        domain: isIp ? '' : value,
        ip: isIp ? value : '',
        port: port ?? 0,
        network,
        inboundTag: inboundTag || '',
        protocol: protocol || '',
      });
      if (msg?.success && msg.obj && typeof msg.obj === 'object') {
        setResult(msg.obj as RouteTestResult);
      }
    } finally {
      setTesting(false);
    }
  }

  const fieldSpan = isMobile ? 24 : undefined;

  return (
    <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
      <Alert type="info" showIcon title={t('pages.xray.routeTesterDesc')} />
      <Row gutter={[8, 8]} align="bottom">
        <Col xs={fieldSpan} sm={7}>
          <Input
            placeholder={t('pages.xray.routeTesterDest')}
            value={dest}
            onChange={(e) => setDest(e.target.value)}
            onPressEnter={run}
            allowClear
          />
        </Col>
        <Col xs={12} sm={3}>
          <InputNumber
            style={{ width: '100%' }}
            min={0}
            max={65535}
            placeholder={t('pages.xray.routeTesterPort')}
            value={port}
            onChange={(v) => setPort(v)}
          />
        </Col>
        <Col xs={12} sm={3}>
          <Select
            style={{ width: '100%' }}
            value={network}
            onChange={setNetwork}
            options={[
              { label: 'TCP', value: 'tcp' },
              { label: 'UDP', value: 'udp' },
            ]}
          />
        </Col>
        <Col xs={12} sm={4}>
          <Select
            style={{ width: '100%' }}
            placeholder={t('pages.xray.routeTesterInbound')}
            allowClear
            value={inboundTag}
            onChange={setInboundTag}
            options={inboundTags.filter(Boolean).map((tag) => ({ label: formatInboundTag(tag, remarkByTag), value: tag }))}
          />
        </Col>
        <Col xs={12} sm={4}>
          <Select
            style={{ width: '100%' }}
            placeholder={t('pages.xray.routeTesterProtocol')}
            allowClear
            value={protocol}
            onChange={setProtocol}
            options={PROTOCOL_OPTIONS}
          />
        </Col>
        <Col xs={fieldSpan} sm={3}>
          <Button type="primary" icon={<AimOutlined />} loading={testing} disabled={!dest.trim()} onClick={run} block>
            {t('pages.xray.routeTesterTest')}
          </Button>
        </Col>
      </Row>

      {result && (
        result.matched ? (
          <Alert
            type="success"
            showIcon
            title={
              <Space wrap>
                <span>{t('pages.xray.routeTesterMatchedOutbound')}:</span>
                <Tag color="blue">{result.outboundTag || '—'}</Tag>
                {(result.groupTags || []).length > 0 && (
                  <>
                    <span>{t('pages.xray.routeTesterViaBalancer')}:</span>
                    {(result.groupTags || []).map((tag) => (
                      <Tag key={tag} color="orange">{tag}</Tag>
                    ))}
                  </>
                )}
              </Space>
            }
          />
        ) : (
          <Alert type="warning" showIcon title={t('pages.xray.routeTesterDefaultOutbound')} />
        )
      )}
    </Space>
  );
}
