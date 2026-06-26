import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Button, Space, Alert, Select, Tooltip } from 'antd';
import { CopyOutlined, QuestionCircleOutlined } from '@ant-design/icons';
import { useState } from 'react';
import ClipboardManager from '@/utils/ClipboardManager';

const UDP_HOP_PATH = ['streamSettings', 'finalmask', 'quicParams', 'udpHop'];

function parsePortRange(rangeStr: string): { start: number; end: number } | null {
  const parts = rangeStr.trim().split('-');
  if (parts.length === 2) {
    const start = parseInt(parts[0].trim(), 10);
    const end = parseInt(parts[1].trim(), 10);
    if (!isNaN(start) && !isNaN(end) && start > 0 && end > 0 && start <= end) {
      return { start, end };
    }
  }
  return null;
}

function generateIptablesRules(basePort: number, portRange: string): string[] {
  const range = parsePortRange(portRange);
  if (!range) return [];

  const rules: string[] = [];
  for (let i = range.start; i <= range.end; i++) {
    rules.push(`iptables -t nat -A PREROUTING -p udp --dport ${i} -j REDIRECT --to-port ${basePort}`);
    rules.push(`ip6tables -t nat -A PREROUTING -p udp --dport ${i} -j REDIRECT --to-port ${basePort}`);
  }
  return rules;
}

function generateUfwRules(basePort: number, portRange: string): string[] {
  const range = parsePortRange(portRange);
  if (!range) return [];

  const rules: string[] = [
    `# Allow base port`,
    `ufw allow ${basePort}/udp`,
  ];

  for (let i = range.start; i <= range.end; i++) {
    if (i !== basePort) {
      rules.push(`ufw allow ${i}/udp`);
    }
  }
  return rules;
}

function generateFirewalldRules(basePort: number, portRange: string): string[] {
  const range = parsePortRange(portRange);
  if (!range) return [];

  const rules: string[] = [];
  for (let i = range.start; i <= range.end; i++) {
    rules.push(
      `firewall-cmd --permanent --add-forward-port=port=${i}:proto=udp:toport=${basePort}`
    );
  }
  rules.push(`firewall-cmd --reload`);
  return rules;
}

function generateNftablesRules(basePort: number, portRange: string): string[] {
  const range = parsePortRange(portRange);
  if (!range) return [];

  const portList = [];
  for (let i = range.start; i <= range.end; i++) {
    portList.push(i);
  }

  return [
    `nft add rule ip nat prerouting udp dport { ${portList.join(
      ', ',
    )} } redirect to ${basePort}`,
    `nft add rule ip6 nat prerouting udp dport { ${portList.join(
      ', ',
    )} } redirect to ${basePort}`,
  ];
}

export default function QuicUdpHopForm({
  basePort,
  form,
}: {
  basePort: number;
  form: any;
}) {
  const { t } = useTranslation();
  const [selectedFirewall, setSelectedFirewall] = useState<string>('iptables');

  const hopConfig = form?.getFieldValue(UDP_HOP_PATH);
  const portRange = hopConfig?.ports || '20000-50000';
  const interval = hopConfig?.interval || '5-10';

  let commands: string[] = [];
  switch (selectedFirewall) {
    case 'ufw':
      commands = generateUfwRules(basePort, portRange);
      break;
    case 'firewalld':
      commands = generateFirewalldRules(basePort, portRange);
      break;
    case 'nftables':
      commands = generateNftablesRules(basePort, portRange);
      break;
    case 'iptables':
    default:
      commands = generateIptablesRules(basePort, portRange);
      break;
  }

  const handleCopy = () => {
    const text = commands.join('\n');
    ClipboardManager.copy(text, t('pages.inbounds.form.portForwardingRulesCopied'));
  };

  const handleToggleUdpHop = (enabled: boolean) => {
    if (enabled) {
      form.setFieldValue(UDP_HOP_PATH, {
        ports: '20000-50000',
        interval: '5-10',
      });
    } else {
      form.setFieldValue(UDP_HOP_PATH, undefined);
    }
  };

  return (
    <>
      <Form.Item label={t('pages.inbounds.form.enableUdpHop')}>
        <Form.Item shouldUpdate noStyle>
          {() => {
            const enabled = !!form?.getFieldValue(UDP_HOP_PATH);
            return (
              <Space>
                <Button
                  type={enabled ? 'primary' : 'default'}
                  onClick={() => handleToggleUdpHop(!enabled)}
                >
                  {enabled ? t('pages.inbounds.form.disableUdpHop') : t('pages.inbounds.form.enableUdpHop')}
                </Button>
                <Tooltip title={t('pages.inbounds.form.udpHopHelp')}>
                  <QuestionCircleOutlined />
                </Tooltip>
              </Space>
            );
          }}
        </Form.Item>
      </Form.Item>

      <Form.Item shouldUpdate noStyle>
        {() => {
          const enabled = !!form?.getFieldValue(UDP_HOP_PATH);
          if (!enabled) return null;

          return (
            <>
              <Form.Item
                label={t('pages.inbounds.form.portRange')}
                name={[...UDP_HOP_PATH, 'ports']}
                rules={[
                  {
                    required: true,
                    message: t('pages.inbounds.form.portRangeRequired'),
                  },
                  {
                    pattern: /^\d+-\d+$/,
                    message: t('pages.inbounds.form.portRangeFormat'),
                  },
                ]}
                tooltip={t('pages.inbounds.form.portRangeExample')}
              >
                <Input placeholder="20000-50000" />
              </Form.Item>

              <Form.Item
                label={t('pages.inbounds.form.hopInterval')}
                name={[...UDP_HOP_PATH, 'interval']}
                rules={[
                  {
                    required: true,
                    message: t('pages.inbounds.form.hopIntervalRequired'),
                  },
                  {
                    pattern: /^\d+-\d+$/,
                    message: t('pages.inbounds.form.hopIntervalFormat'),
                  },
                ]}
                tooltip={t('pages.inbounds.form.hopIntervalExample')}
              >
                <Input placeholder="5-10" />
              </Form.Item>

              <Form.Item label={t('pages.inbounds.form.portForwardingRules')}>
                <Alert
                  type="info"
                  message={t('pages.inbounds.form.portForwardingInfo')}
                  showIcon
                  style={{ marginBottom: '12px' }}
                />

                <Form.Item
                  label={t('pages.inbounds.form.firewallType')}
                  noStyle
                >
                  <Select
                    style={{ marginBottom: '12px' }}
                    value={selectedFirewall}
                    onChange={setSelectedFirewall}
                    options={[
                      { value: 'iptables', label: 'iptables (Linux)' },
                      { value: 'ufw', label: 'UFW (Debian/Ubuntu)' },
                      { value: 'firewalld', label: 'firewalld (RHEL/CentOS)' },
                      { value: 'nftables', label: 'nftables (Modern Linux)' },
                    ]}
                  />
                </Form.Item>

                {commands.length > 0 && (
                  <>
                    <pre
                      style={{
                        backgroundColor: '#f5f5f5',
                        padding: '12px',
                        borderRadius: '4px',
                        maxHeight: '200px',
                        overflow: 'auto',
                        fontSize: '12px',
                        fontFamily: 'monospace',
                      }}
                    >
                      {commands.join('\n')}
                    </pre>
                    <Button
                      type="primary"
                      icon={<CopyOutlined />}
                      onClick={handleCopy}
                      style={{ marginTop: '8px' }}
                    >
                      {t('pages.inbounds.form.copyRules')}
                    </Button>
                  </>
                )}
              </Form.Item>
            </>
          );
        }}
      </Form.Item>
    </>
  );
}
