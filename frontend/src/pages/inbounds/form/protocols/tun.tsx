import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Select, Space, Switch, Tooltip } from 'antd';
import { MinusOutlined, PlusOutlined } from '@ant-design/icons';
import { useFieldArray, useFormContext } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';

interface StringListProps {
  name: string[];
  label: ReactNode;
  placeholder: (index: number) => string;
}

function StringList({ name, label, placeholder }: StringListProps) {
  const { t } = useTranslation();
  const { control } = useFormContext();
  const { fields, append, remove } = useFieldArray({ control, name: name.join('.') });
  return (
    <Form.Item label={label}>
      <Button aria-label={t('add')} size="small" onClick={() => append('')}>
        <PlusOutlined />
      </Button>
      {fields.map((field, j) => (
        <Space.Compact key={field.id} block className="mt-4">
          <FormField name={[...name, j]} noStyle>
            <Input placeholder={placeholder(j)} />
          </FormField>
          <Button aria-label={t('remove')} size="small" onClick={() => remove(j)}>
            <MinusOutlined />
          </Button>
        </Space.Compact>
      ))}
    </Form.Item>
  );
}

// xray-core 26.9.30 accepts exactly these autoSystemWfpBlockLeak values.
const WFP_BLOCK_LEAK_OPTIONS = [
  { value: 'dns', label: 'dns' },
  { value: 'misconfigtun', label: 'misconfigtun' },
];

export default function TunFields() {
  const { t } = useTranslation();
  return (
    <>
      <FormField name={['settings', 'name']} label={t('pages.inbounds.info.interfaceName')}>
        <Input placeholder="xray0" />
      </FormField>
      <FormField
        name={['settings', 'desc']}
        label={t('pages.inbounds.form.tunDesc')}
        tooltip={t('pages.inbounds.form.tunDescTooltip')}
      >
        <Input id="tun-desc" placeholder="Wintun" />
      </FormField>
      <FormField name={['settings', 'mtu']} label="MTU">
        <InputNumber min={0} />
      </FormField>
      <StringList
        name={['settings', 'gateway']}
        label={t('pages.inbounds.info.gateway')}
        placeholder={(j) => (j === 0 ? '10.0.0.1/16' : 'fc00::1/64')}
      />
      <StringList
        name={['settings', 'dns']}
        label="DNS"
        placeholder={(j) => (j === 0 ? '1.1.1.1' : '8.8.8.8')}
      />
      <FormField name={['settings', 'userLevel']} label={t('pages.xray.tun.userLevel')}>
        <InputNumber min={0} />
      </FormField>
      <StringList
        name={['settings', 'autoSystemRoutingTable']}
        label={
          <Tooltip title={t('pages.inbounds.form.autoSystemRoutesTooltip')}>
            {t('pages.inbounds.info.autoSystemRoutes')}
          </Tooltip>
        }
        placeholder={(j) => (j === 0 ? '0.0.0.0/0' : '::/0')}
      />
      <FormField
        name={['settings', 'autoOutboundsInterface']}
        label={
          <Tooltip title={t('pages.inbounds.form.autoOutboundsInterfaceTooltip')}>
            {t('pages.inbounds.form.autoOutboundsInterface')}
          </Tooltip>
        }
      >
        <Input placeholder="auto" />
      </FormField>
      <FormField
        name={['settings', 'autoSystemDnsToGateway']}
        label={t('pages.inbounds.form.autoSystemDnsToGateway')}
        tooltip={t('pages.inbounds.form.autoSystemDnsToGatewayTooltip')}
        valueProp="checked"
      >
        <Switch id="tun-auto-dns-gateway" />
      </FormField>
      <FormField
        name={['settings', 'autoSystemWfpBlockLeak']}
        label={t('pages.inbounds.form.autoSystemWfpBlockLeak')}
        tooltip={t('pages.inbounds.form.autoSystemWfpBlockLeakTooltip')}
      >
        <Select
          id="tun-wfp-block-leak"
          mode="multiple"
          allowClear
          options={WFP_BLOCK_LEAK_OPTIONS}
        />
      </FormField>
    </>
  );
}
