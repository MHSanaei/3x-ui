import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Form, Input, InputNumber, Select, Space, Switch } from 'antd';
import { RadarChartOutlined, ReloadOutlined, ThunderboltOutlined, ToolOutlined } from '@ant-design/icons';

import { FormField } from '@/components/form/rhf';
import { CPS_SLOTS, I1_PROFILE_CHOICES, type CpsSlot, type I1ProfileChoice } from '@/lib/xray/i1Generators';
import { AWG_VERSION_2, AWG_VERSION_3 } from '@/schemas/protocols/inbound/amneziawg';
import AwgDiagnosticsModal from '@/pages/inbounds/form/AwgDiagnosticsModal';

interface AmneziawgFieldsProps {
  awgPubKey: string;
  inboundId: number | null;
  regenInboundAwg: () => void;
  regenInboundAwgObfuscation: () => void;
  regenInboundAwgHeaderProtectionKey: () => void;
  regenInboundAwgContentPaddingAddition: () => void;
  onAwgVersionChange: (version: string) => void;
  regenInboundAwgRekeyAfterTime: () => void;
  regenInboundAwgRekeyTimeout: () => void;
  regenInboundAwgRejectAfterTime: () => void;
  regenInboundAwgKeepaliveTimeout: () => void;
  regenInboundAwgMaxHandshakeAttempts: () => void;
  // One mimicry-profile choice per CPS slot (i1-i5) -- keyed by CpsSlot so a
  // single generate row (see the CPS_SLOTS.map below) covers all five.
  cpsProfiles: Record<CpsSlot, I1ProfileChoice>;
  onCpsProfileChange: (slot: CpsSlot, profile: I1ProfileChoice) => void;
  onRegenCps: (slot: CpsSlot) => void;
  quicCapturing: boolean;
  onQuicCapture: (host: string, slot: CpsSlot) => void;
  onQuicCaptureAll: (host: string) => void;
}

export default function AmneziawgFields({
  awgPubKey,
  inboundId,
  regenInboundAwg,
  regenInboundAwgObfuscation,
  regenInboundAwgHeaderProtectionKey,
  regenInboundAwgContentPaddingAddition,
  onAwgVersionChange,
  regenInboundAwgRekeyAfterTime,
  regenInboundAwgRekeyTimeout,
  regenInboundAwgRejectAfterTime,
  regenInboundAwgKeepaliveTimeout,
  regenInboundAwgMaxHandshakeAttempts,
  cpsProfiles,
  onCpsProfileChange,
  onRegenCps,
  quicCapturing,
  onQuicCapture,
  onQuicCaptureAll,
}: AmneziawgFieldsProps) {
  const { t } = useTranslation();
  const [diagnosticsOpen, setDiagnosticsOpen] = useState(false);
  const [quicCaptureHost, setQuicCaptureHost] = useState('');
  const [quicCaptureSlot, setQuicCaptureSlot] = useState<CpsSlot>('i1');
  return (
    <>
      <Form.Item label={t('pages.xray.amneziawg.privateKey')}>
        <Space.Compact block>
          <FormField name={['settings', 'server', 'privateKey']} noStyle>
            <Input style={{ width: 'calc(100% - 32px)' }} />
          </FormField>
          <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={regenInboundAwg} />
        </Space.Compact>
      </Form.Item>
      <Form.Item label={t('pages.xray.amneziawg.publicKey')}>
        <Input value={awgPubKey} disabled />
      </Form.Item>
      <Form.Item label={t('pages.xray.amneziawg.diagnostics')} tooltip={t('pages.xray.amneziawg.diagnosticsHint')}>
        <Button icon={<ToolOutlined />} disabled={!inboundId} onClick={() => setDiagnosticsOpen(true)}>
          {t('pages.xray.amneziawg.diagnosticsOpen')}
        </Button>
      </Form.Item>
      {inboundId && (
        <AwgDiagnosticsModal
          open={diagnosticsOpen}
          onClose={() => setDiagnosticsOpen(false)}
          inboundId={inboundId}
        />
      )}
      <FormField name={['settings', 'server', 'subnetIp']} label={t('pages.xray.amneziawg.subnetIp')}>
        <Input placeholder="10.8.1.0" />
      </FormField>
      <FormField name={['settings', 'server', 'subnetCidr']} label={t('pages.xray.amneziawg.subnetCidr')}>
        <InputNumber min={1} max={32} style={{ width: '100%' }} />
      </FormField>
      <FormField name={['settings', 'server', 'mtu']} label={t('pages.xray.amneziawg.mtu')}>
        <InputNumber style={{ width: '100%' }} />
      </FormField>
      <FormField name={['settings', 'server', 'primaryDns']} label={t('pages.xray.amneziawg.primaryDns')}>
        <Input placeholder="8.8.8.8" />
      </FormField>
      <FormField name={['settings', 'server', 'secondaryDns']} label={t('pages.xray.amneziawg.secondaryDns')}>
        <Input placeholder="8.8.4.4" />
      </FormField>
      <FormField
        name={['settings', 'server', 'externalInterface']}
        label={t('pages.xray.amneziawg.externalInterface')}
        tooltip={t('pages.xray.amneziawg.externalInterfaceHint')}
      >
        <Input placeholder="eth0" />
      </FormField>
      <FormField
        name={['settings', 'server', 'ipv6Enabled']}
        label={t('pages.xray.amneziawg.ipv6Enabled')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField
        name={['settings', 'server', 'ipv6Subnet']}
        label={t('pages.xray.amneziawg.ipv6Subnet')}
        tooltip={t('pages.xray.amneziawg.ipv6SubnetHint')}
      >
        <Input placeholder="fd86:ea04:1115::/64" />
      </FormField>
      <FormField
        name={['settings', 'server', 'ipv6ExternalInterface']}
        label={t('pages.xray.amneziawg.ipv6ExternalInterface')}
        tooltip={t('pages.xray.amneziawg.ipv6ExternalInterfaceHint')}
      >
        <Input placeholder="eth0" />
      </FormField>
      <Form.Item label={t('pages.xray.amneziawg.obfuscation')} tooltip={t('pages.xray.amneziawg.obfuscationHint')}>
        <Button icon={<ReloadOutlined />} onClick={regenInboundAwgObfuscation}>
          {t('pages.xray.amneziawg.regenerateObfuscation')}
        </Button>
      </Form.Item>
      <FormField name={['settings', 'server', 'jc']} label={t('pages.xray.amneziawg.jc')}>
        <InputNumber min={0} style={{ width: '100%' }} />
      </FormField>
      <FormField name={['settings', 'server', 'jmin']} label={t('pages.xray.amneziawg.jmin')}>
        <InputNumber min={0} style={{ width: '100%' }} />
      </FormField>
      <FormField name={['settings', 'server', 'jmax']} label={t('pages.xray.amneziawg.jmax')}>
        <InputNumber min={0} style={{ width: '100%' }} />
      </FormField>
      <FormField name={['settings', 'server', 's1']} label={t('pages.xray.amneziawg.s1')}>
        <InputNumber min={0} style={{ width: '100%' }} />
      </FormField>
      <FormField name={['settings', 'server', 's2']} label={t('pages.xray.amneziawg.s2')}>
        <InputNumber min={0} style={{ width: '100%' }} />
      </FormField>
      <FormField name={['settings', 'server', 's3']} label={t('pages.xray.amneziawg.s3')}>
        <InputNumber min={0} max={64} style={{ width: '100%' }} />
      </FormField>
      <FormField name={['settings', 'server', 's4']} label={t('pages.xray.amneziawg.s4')}>
        <InputNumber min={0} max={32} style={{ width: '100%' }} />
      </FormField>
      <FormField
        name={['settings', 'server', 'h1']}
        label={t('pages.xray.amneziawg.h1')}
        tooltip={t('pages.xray.amneziawg.hHint')}
      >
        <Input placeholder="1 or 100-800" />
      </FormField>
      <FormField name={['settings', 'server', 'h2']} label={t('pages.xray.amneziawg.h2')}>
        <Input placeholder="2 or 100-800" />
      </FormField>
      <FormField name={['settings', 'server', 'h3']} label={t('pages.xray.amneziawg.h3')}>
        <Input placeholder="3 or 100-800" />
      </FormField>
      <FormField name={['settings', 'server', 'h4']} label={t('pages.xray.amneziawg.h4')}>
        <Input placeholder="4 or 100-800" />
      </FormField>
      <Form.Item label={t('pages.xray.amneziawg.quicCapture')} tooltip={t('pages.xray.amneziawg.quicCaptureHint')}>
        <Space direction="vertical" style={{ width: '100%' }} size={8}>
          <Input
            value={quicCaptureHost}
            onChange={(e) => setQuicCaptureHost(e.target.value)}
            placeholder="www.google.com"
          />
          <Space.Compact block style={{ display: 'flex' }}>
            <Select
              aria-label={t('pages.xray.amneziawg.quicCaptureSlot')}
              value={quicCaptureSlot}
              onChange={setQuicCaptureSlot}
              style={{ width: 90 }}
              options={CPS_SLOTS.map((slot) => ({ value: slot, label: slot.toUpperCase() }))}
            />
            <Button
              loading={quicCapturing}
              icon={<RadarChartOutlined />}
              onClick={() => onQuicCapture(quicCaptureHost, quicCaptureSlot)}
              style={{ flex: 1 }}
            >
              {t('pages.xray.amneziawg.quicCaptureButton')}
            </Button>
            <Button
              loading={quicCapturing}
              icon={<ThunderboltOutlined />}
              onClick={() => onQuicCaptureAll(quicCaptureHost)}
              style={{ flex: 1 }}
            >
              {t('pages.xray.amneziawg.quicCaptureAllButton')}
            </Button>
          </Space.Compact>
        </Space>
      </Form.Item>
      {CPS_SLOTS.map((slot, index) => (
        <Form.Item
          key={slot}
          label={t(`pages.xray.amneziawg.${slot}`)}
          // The first slot explains *which profile to pick and why* (right
          // where the profile Select first appears), the last slot closes
          // the group with the "AmneziaWG 2.0 only" compatibility note --
          // I2-I4 repeat neither, same "explain once" convention as H1-H4.
          tooltip={
            index === 0
              ? t('pages.xray.amneziawg.i1ProfileGuidance')
              : index === CPS_SLOTS.length - 1
                ? t('pages.xray.amneziawg.i1Hint')
                : undefined
          }
        >
          <Space.Compact block style={{ display: 'flex' }}>
            <Select
              aria-label={t('pages.xray.amneziawg.i1Profile')}
              value={cpsProfiles[slot]}
              onChange={(profile) => onCpsProfileChange(slot, profile)}
              style={{ width: 120 }}
              options={I1_PROFILE_CHOICES.map((profile) => ({
                value: profile,
                label: t(`pages.xray.amneziawg.i1ProfileOptions.${profile}`),
              }))}
            />
            <FormField name={['settings', 'server', slot]} noStyle>
              <Input placeholder="<r 64>" style={{ flex: 1 }} />
            </FormField>
            <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={() => onRegenCps(slot)} />
          </Space.Compact>
        </Form.Item>
      ))}
      <Divider style={{ margin: '0 0 14px 0' }}>{t('pages.xray.amneziawg.awg3Advanced')}</Divider>
      <FormField
        name={['settings', 'server', 'awgVersion']}
        label={t('pages.xray.amneziawg.awgVersion')}
        tooltip={t('pages.xray.amneziawg.awgVersionHint')}
        onAfterChange={(value) => onAwgVersionChange(value as string)}
      >
        <Select
          options={[
            { value: AWG_VERSION_2, label: t('pages.xray.amneziawg.awgVersionOptions.2') },
            { value: AWG_VERSION_3, label: t('pages.xray.amneziawg.awgVersionOptions.3') },
          ]}
        />
      </FormField>
      <Form.Item
        label={t('pages.xray.amneziawg.headerProtectionKey')}
        tooltip={t('pages.xray.amneziawg.headerProtectionKeyHint')}
      >
        <Space.Compact block>
          <FormField name={['settings', 'server', 'headerProtectionKey']} noStyle>
            <Input style={{ width: 'calc(100% - 32px)' }} />
          </FormField>
          <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={regenInboundAwgHeaderProtectionKey} />
        </Space.Compact>
      </Form.Item>
      <Form.Item
        label={t('pages.xray.amneziawg.contentPaddingAddition')}
        tooltip={t('pages.xray.amneziawg.contentPaddingAdditionHint')}
      >
        <Space.Compact block>
          <FormField name={['settings', 'server', 'contentPaddingAddition']} noStyle>
            <Input placeholder="0 or 50-100" style={{ width: 'calc(100% - 32px)' }} />
          </FormField>
          <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={regenInboundAwgContentPaddingAddition} />
        </Space.Compact>
      </Form.Item>
      <Form.Item
        label={t('pages.xray.amneziawg.rekeyAfterTime')}
        tooltip={t('pages.xray.amneziawg.rekeyAfterTimeHint')}
      >
        <Space.Compact block>
          <FormField name={['settings', 'server', 'rekeyAfterTime']} noStyle>
            <Input placeholder="120 or 118-135" style={{ width: 'calc(100% - 32px)' }} />
          </FormField>
          <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={regenInboundAwgRekeyAfterTime} />
        </Space.Compact>
      </Form.Item>
      <Form.Item
        label={t('pages.xray.amneziawg.rekeyTimeout')}
        tooltip={t('pages.xray.amneziawg.rekeyTimeoutHint')}
      >
        <Space.Compact block>
          <FormField name={['settings', 'server', 'rekeyTimeout']} noStyle>
            <Input placeholder="5 or 4-8" style={{ width: 'calc(100% - 32px)' }} />
          </FormField>
          <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={regenInboundAwgRekeyTimeout} />
        </Space.Compact>
      </Form.Item>
      <Form.Item
        label={t('pages.xray.amneziawg.rejectAfterTime')}
        tooltip={t('pages.xray.amneziawg.rejectAfterTimeHint')}
      >
        <Space.Compact block>
          <FormField name={['settings', 'server', 'rejectAfterTime']} noStyle>
            <Input placeholder="180 or 175-190" style={{ width: 'calc(100% - 32px)' }} />
          </FormField>
          <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={regenInboundAwgRejectAfterTime} />
        </Space.Compact>
      </Form.Item>
      <Form.Item
        label={t('pages.xray.amneziawg.keepaliveTimeout')}
        tooltip={t('pages.xray.amneziawg.keepaliveTimeoutHint')}
      >
        <Space.Compact block>
          <FormField name={['settings', 'server', 'keepaliveTimeout']} noStyle>
            <Input placeholder="10 or 9-17" style={{ width: 'calc(100% - 32px)' }} />
          </FormField>
          <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={regenInboundAwgKeepaliveTimeout} />
        </Space.Compact>
      </Form.Item>
      <Form.Item
        label={t('pages.xray.amneziawg.maxHandshakeAttempts')}
        tooltip={t('pages.xray.amneziawg.maxHandshakeAttemptsHint')}
      >
        <Space.Compact block>
          <FormField name={['settings', 'server', 'maxHandshakeAttempts']} noStyle>
            <Input placeholder="18 or 15-22" style={{ width: 'calc(100% - 32px)' }} />
          </FormField>
          <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={regenInboundAwgMaxHandshakeAttempts} />
        </Space.Compact>
      </Form.Item>
      <FormField
        name={['settings', 'server', 'randomTrailers']}
        label={t('pages.xray.amneziawg.randomTrailers')}
        tooltip={t('pages.xray.amneziawg.randomTrailersHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField
        name={['settings', 'server', 'disableCookies']}
        label={t('pages.xray.amneziawg.disableCookies')}
        tooltip={t('pages.xray.amneziawg.disableCookiesHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
    </>
  );
}
