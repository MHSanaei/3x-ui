import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Select, Space, Switch } from 'antd';
import { ReloadOutlined, ToolOutlined } from '@ant-design/icons';

import { FormField } from '@/components/form/rhf';
import { I1_PROFILE_CHOICES, type I1ProfileChoice } from '@/lib/xray/i1Generators';
import { AWG_VERSION_2, AWG_VERSION_3 } from '@/schemas/protocols/inbound/amneziawg';
import AwgDiagnosticsModal from '@/pages/inbounds/form/AwgDiagnosticsModal';

interface AmneziawgFieldsProps {
  awgPubKey: string;
  inboundId: number | null;
  regenInboundAwg: () => void;
  regenInboundAwgObfuscation: () => void;
  regenInboundAwgHeaderProtectionKey: () => void;
  regenInboundAwgContentPaddingAddition: () => void;
  i1Profile: I1ProfileChoice;
  onI1ProfileChange: (profile: I1ProfileChoice) => void;
  regenInboundAwgI1: () => void;
}

export default function AmneziawgFields({
  awgPubKey,
  inboundId,
  regenInboundAwg,
  regenInboundAwgObfuscation,
  regenInboundAwgHeaderProtectionKey,
  regenInboundAwgContentPaddingAddition,
  i1Profile,
  onI1ProfileChange,
  regenInboundAwgI1,
}: AmneziawgFieldsProps) {
  const { t } = useTranslation();
  const [diagnosticsOpen, setDiagnosticsOpen] = useState(false);
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
      <Form.Item label={t('pages.xray.amneziawg.diagnostics')} extra={t('pages.xray.amneziawg.diagnosticsHint')}>
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
        extra={t('pages.xray.amneziawg.externalInterfaceHint')}
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
        extra={t('pages.xray.amneziawg.ipv6SubnetHint')}
      >
        <Input placeholder="fd86:ea04:1115::/64" />
      </FormField>
      <FormField
        name={['settings', 'server', 'ipv6ExternalInterface']}
        label={t('pages.xray.amneziawg.ipv6ExternalInterface')}
        extra={t('pages.xray.amneziawg.ipv6ExternalInterfaceHint')}
      >
        <Input placeholder="eth0" />
      </FormField>
      <Form.Item label={t('pages.xray.amneziawg.obfuscation')} extra={t('pages.xray.amneziawg.obfuscationHint')}>
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
        extra={t('pages.xray.amneziawg.hHint')}
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
      <Form.Item
        label={t('pages.xray.amneziawg.i1')}
        extra={t('pages.xray.amneziawg.i1Hint')}
      >
        <Space.Compact block style={{ display: 'flex' }}>
          <Select
            aria-label={t('pages.xray.amneziawg.i1Profile')}
            value={i1Profile}
            onChange={onI1ProfileChange}
            style={{ width: 120 }}
            options={I1_PROFILE_CHOICES.map((profile) => ({
              value: profile,
              label: t(`pages.xray.amneziawg.i1ProfileOptions.${profile}`),
            }))}
          />
          <FormField name={['settings', 'server', 'i1']} noStyle>
            <Input placeholder="<r 64>" style={{ flex: 1 }} />
          </FormField>
          <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={regenInboundAwgI1} />
        </Space.Compact>
      </Form.Item>
      <FormField
        name={['settings', 'server', 'awgVersion']}
        label={t('pages.xray.amneziawg.awgVersion')}
        extra={t('pages.xray.amneziawg.awgVersionHint')}
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
        extra={t('pages.xray.amneziawg.headerProtectionKeyHint')}
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
        extra={t('pages.xray.amneziawg.contentPaddingAdditionHint')}
      >
        <Space.Compact block>
          <FormField name={['settings', 'server', 'contentPaddingAddition']} noStyle>
            <Input placeholder="0 or 50-100" style={{ width: 'calc(100% - 32px)' }} />
          </FormField>
          <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={regenInboundAwgContentPaddingAddition} />
        </Space.Compact>
      </Form.Item>
    </>
  );
}
