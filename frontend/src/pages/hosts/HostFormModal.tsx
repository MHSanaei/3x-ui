import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Modal, Select, Switch, Tabs, message } from 'antd';
import {
  ProfileOutlined,
  SafetyCertificateOutlined,
  ControlOutlined,
  NodeIndexOutlined,
  SettingOutlined,
  PartitionOutlined,
  DeploymentUnitOutlined,
  RocketOutlined,
} from '@ant-design/icons';
import { Controller, FormProvider, useForm, useWatch } from 'react-hook-form';

import type { HostRecord } from '@/api/queries/useHostsQuery';
import { BulkAddHostSchema, type BulkAddHostValues } from '@/schemas/api/host';
import type { InboundOption } from '@/schemas/client';
import { ALPN_OPTION, UTLS_FINGERPRINT } from '@/schemas/primitives';
import { FormField, rhfZodValidate } from '@/components/form/rhf';
import { useNodesQuery } from '@/api/queries/useNodesQuery';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from '@/pages/settings/catTabLabel';
import { HostFinalMaskForm, HostMuxForm, HostSockoptForm } from './json-forms';

type FormShape = Omit<BulkAddHostValues, 'isDisabled'> & {
  enable: boolean;
};

interface HostFormModalProps {
  open: boolean;
  mode: 'add' | 'edit';
  host: HostRecord | null;
  inboundOptions: InboundOption[];
  existingHosts: HostRecord[];
  save: (payload: BulkAddHostValues) => Promise<{ success?: boolean; msg?: string } | undefined>;
  onOpenChange: (open: boolean) => void;
}

const asString = (v: unknown): string => (typeof v === 'string' ? v : '');

function defaultsFor(host: HostRecord | null): FormShape {
  return {
    inboundIds: host?.inboundIds ?? [],
    hosts: (host?.hosts || []).filter((h) => h && h.trim() !== ''),
    sortOrder: host?.sortOrder ?? 0,
    remark: host?.remark ?? '',
    serverDescription: host?.serverDescription ?? '',
    enable: host ? !host.isDisabled : true,
    isHidden: host?.isHidden ?? false,
    tags: host?.tags ?? [],
    port: host?.port ?? 0,
    security: (host?.security as BulkAddHostValues['security']) ?? 'same',
    sni: host?.sni ?? '',
    hostHeader: host?.hostHeader ?? '',
    path: host?.path ?? '',
    alpn: (host?.alpn as BulkAddHostValues['alpn']) ?? [],
    fingerprint: host?.fingerprint as BulkAddHostValues['fingerprint'],
    overrideSniFromAddress: host?.overrideSniFromAddress ?? false,
    keepSniBlank: host?.keepSniBlank ?? false,
    pinnedPeerCertSha256: host?.pinnedPeerCertSha256 ?? [],
    verifyPeerCertByName: (host?.verifyPeerCertByName as string | undefined) ?? '',
    allowInsecure: host?.allowInsecure ?? false,
    echConfigList: host?.echConfigList ?? '',
    muxParams: asString(host?.muxParams),
    sockoptParams: asString(host?.sockoptParams),
    finalMask: host?.finalMask ?? '',
    vlessRoute: host?.vlessRoute ?? '',
    excludeFromSubTypes: (host?.excludeFromSubTypes as BulkAddHostValues['excludeFromSubTypes']) ?? [],
    nodeGuids: host?.nodeGuids ?? [],
    mihomoIpVersion: host?.mihomoIpVersion as BulkAddHostValues['mihomoIpVersion'],
    mihomoX25519: host?.mihomoX25519 ?? false,
    shuffleHost: host?.shuffleHost ?? false,
  };
}

export default function HostFormModal({ open, mode, host, inboundOptions, existingHosts, save, onOpenChange }: HostFormModalProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const methods = useForm<FormShape>({ defaultValues: defaultsFor(host) });
  const [messageApi, messageContextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);

  const security = (useWatch({ control: methods.control, name: 'security' }) ?? 'same') as string;
  const showTls = security === 'tls' || security === 'reality';
  const showTlsExtras = security === 'tls';

  useEffect(() => {
    if (open) {
      methods.reset(defaultsFor(host));
      setLoading(false);
    }
  }, [open, host, methods]);

  const { nodes } = useNodesQuery();

  const inboundSelectOptions = useMemo(
    () => inboundOptions.map((ib) => ({
      value: ib.id,
      label: ib.remark || ib.tag || `#${ib.id}`,
    })),
    [inboundOptions],
  );

  const nodeSelectOptions = useMemo(
    () => nodes
      .filter((n) => n.guid)
      .map((n) => ({ value: n.guid as string, label: n.name || n.remark || (n.guid as string) })),
    [nodes],
  );

  const alpnOptions = useMemo(() => Object.values(ALPN_OPTION).map((v) => ({ value: v, label: v })), []);
  const fpOptions = useMemo(() => Object.values(UTLS_FINGERPRINT).map((v) => ({ value: v, label: v })), []);

  const hostOptions = useMemo(() => {
    const addresses = new Set<string>();
    for (const h of existingHosts || []) {
      if (h.hosts) {
        for (const addr of h.hosts) {
          if (addr && addr.trim() !== '') {
            addresses.add(addr);
          }
        }
      }
    }
    return Array.from(addresses).map((addr) => ({ value: addr, label: addr }));
  }, [existingHosts]);

  const onFinish = async (values: FormShape) => {
    if (loading) return;
    const { enable, ...rest } = values;
    const isDisabled = !enable;
    const payload: BulkAddHostValues = {
      ...rest,
      hosts: (rest.hosts || []).filter((h) => h && h.trim() !== ''),
      isDisabled,
    };
    setLoading(true);
    try {
      const res = await save(payload);
      if (res?.success) {
        messageApi.success(t(mode === 'add' ? 'pages.hosts.toasts.add' : 'pages.hosts.toasts.update'));
        onOpenChange(false);
      } else if (res?.msg) {
        messageApi.error(res.msg);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      open={open}
      title={t(mode === 'add' ? 'pages.hosts.addHost' : 'pages.hosts.editHost')}
      onOk={methods.handleSubmit(onFinish)}
      onCancel={() => onOpenChange(false)}
      confirmLoading={loading}
      okText={t('save')}
      cancelText={t('cancel')}
      destroyOnHidden
      width={isMobile ? '95vw' : 760}
      styles={{ body: { maxHeight: '70vh', overflowY: 'auto', overflowX: 'hidden' } }}
    >
      {messageContextHolder}
      <FormProvider {...methods}>
        <Form
          colon={false}
          labelCol={{ sm: { span: 8 } }}
          wrapperCol={{ sm: { span: 14 } }}
          labelWrap
        >
          <Tabs
            defaultActiveKey="basic"
            items={[
              {
                key: 'basic',
                forceRender: true,
                label: catTabLabel(<ProfileOutlined />, t('pages.hosts.sections.basic'), isMobile),
                children: (
                  <>
                    <FormField name="remark" label={t('pages.hosts.fields.remark')} tooltip={t('pages.hosts.hints.remark')} rules={{ validate: rhfZodValidate(BulkAddHostSchema.shape.remark) }}>
                      <Input maxLength={256} />
                    </FormField>
                    <FormField name="serverDescription" label={t('pages.hosts.fields.serverDescription')} tooltip={t('pages.hosts.hints.serverDescription')}>
                      <Input maxLength={64} />
                    </FormField>
                    <FormField name="inboundIds" label={t('pages.hosts.fields.inbound')} rules={{ validate: rhfZodValidate(BulkAddHostSchema.shape.inboundIds) }}>
                      <Select
                        mode="multiple"
                        options={inboundSelectOptions}
                        showSearch
                        optionFilterProp="label"
                        placeholder={t('pages.hosts.selectInbound')}
                      />
                    </FormField>
                    <FormField name="hosts" label={t('pages.hosts.fields.address')} tooltip={t('pages.hosts.hints.address')} rules={{ validate: rhfZodValidate(BulkAddHostSchema.shape.hosts) }}>
                      <Select
                        mode="tags"
                        options={hostOptions}
                        tokenSeparators={[',', ';', ' ']}
                        placeholder="cdn.example.com, cdn2.example.com:443"
                      />
                    </FormField>
                    <FormField name="port" label={t('pages.hosts.fields.port')} tooltip={t('pages.hosts.hints.port')}>
                      <InputNumber min={0} max={65535} />
                    </FormField>
                    <FormField name="tags" label={t('pages.hosts.fields.tags')} tooltip={t('pages.hosts.hints.tags')}>
                      <Select mode="tags" allowClear tokenSeparators={[',']} />
                    </FormField>
                    <FormField name="nodeGuids" label={t('pages.hosts.fields.nodeGuids')} tooltip={t('pages.hosts.hints.nodeGuids')}>
                      <Select mode="multiple" allowClear options={nodeSelectOptions} optionFilterProp="label" />
                    </FormField>
                    <FormField name="enable" label={t('pages.hosts.fields.enable')} valueProp="checked">
                      <Switch />
                    </FormField>
                  </>
                ),
              },
              {
                key: 'security',
                forceRender: true,
                label: catTabLabel(<SafetyCertificateOutlined />, t('pages.hosts.sections.security'), isMobile),
                children: (
                  <>
                    <FormField name="security" label={t('pages.hosts.fields.security')}>
                      <Select
                        options={['same', 'tls', 'none', 'reality'].map((v) => ({ value: v, label: v }))}
                      />
                    </FormField>
                    {showTls && (
                      <>
                        <FormField name="sni" label={t('pages.hosts.fields.sni')}>
                          <Input />
                        </FormField>
                        <FormField name="overrideSniFromAddress" label={t('pages.hosts.fields.overrideSniFromAddress')} valueProp="checked">
                          <Switch />
                        </FormField>
                        <FormField name="keepSniBlank" label={t('pages.hosts.fields.keepSniBlank')} valueProp="checked">
                          <Switch />
                        </FormField>
                        <FormField name="fingerprint" label={t('pages.hosts.fields.fingerprint')}>
                          <Select allowClear options={fpOptions} />
                        </FormField>
                      </>
                    )}
                    {showTlsExtras && (
                      <>
                        <FormField name="alpn" label={t('pages.hosts.fields.alpn')}>
                          <Select mode="multiple" allowClear options={alpnOptions} />
                        </FormField>
                        <FormField name="pinnedPeerCertSha256" label={t('pages.hosts.fields.pins')}>
                          <Select mode="tags" allowClear tokenSeparators={[',']} />
                        </FormField>
                        <FormField name="verifyPeerCertByName" label={t('pages.hosts.fields.verifyPeerCertByName')} tooltip={t('pages.inbounds.form.verifyPeerCertByNameTip')}>
                          <Input placeholder="example.com" />
                        </FormField>
                        <FormField name="allowInsecure" label={t('pages.hosts.fields.allowInsecure')} tooltip={t('pages.hosts.hints.allowInsecure')} valueProp="checked">
                          <Switch />
                        </FormField>
                        <FormField name="echConfigList" label={t('pages.hosts.fields.echConfigList')}>
                          <Input.TextArea rows={2} />
                        </FormField>
                      </>
                    )}
                  </>
                ),
              },
              {
                key: 'advanced',
                forceRender: true,
                label: catTabLabel(<ControlOutlined />, t('pages.hosts.sections.advanced'), isMobile),
                children: (
                  <Tabs
                    size="small"
                    defaultActiveKey="adv-general"
                    items={[
                      {
                        key: 'adv-general',
                        forceRender: true,
                        label: catTabLabel(<SettingOutlined />, t('pages.hosts.sections.general'), isMobile),
                        children: (
                          <>
                            <FormField name="hostHeader" label={t('pages.hosts.fields.hostHeader')}>
                              <Input />
                            </FormField>
                            <FormField name="path" label={t('pages.hosts.fields.path')}>
                              <Input />
                            </FormField>
                            <FormField name="vlessRoute" label={t('pages.hosts.fields.vlessRoute')} tooltip={t('pages.hosts.hints.vlessRoute')}>
                              <Input placeholder="443" />
                            </FormField>
                            <FormField name="excludeFromSubTypes" label={t('pages.hosts.fields.excludeFromSubTypes')}>
                              <Select
                                mode="multiple"
                                allowClear
                                options={['raw', 'json', 'clash'].map((v) => ({ value: v, label: v }))}
                              />
                            </FormField>
                          </>
                        ),
                      },
                      {
                        key: 'adv-mux',
                        forceRender: true,
                        label: catTabLabel(<PartitionOutlined />, t('pages.hosts.fields.muxParams'), isMobile),
                        children: (
                          <Form.Item noStyle>
                            <Controller
                              control={methods.control}
                              name="muxParams"
                              render={({ field }) => (
                                <HostMuxForm value={field.value} onChange={field.onChange} />
                              )}
                            />
                          </Form.Item>
                        ),
                      },
                      {
                        key: 'adv-sockopt',
                        forceRender: true,
                        label: catTabLabel(<DeploymentUnitOutlined />, t('pages.hosts.fields.sockoptParams'), isMobile),
                        children: (
                          <Form.Item noStyle>
                            <Controller
                              control={methods.control}
                              name="sockoptParams"
                              render={({ field }) => (
                                <HostSockoptForm value={field.value} onChange={field.onChange} />
                              )}
                            />
                          </Form.Item>
                        ),
                      },
                      {
                        key: 'adv-finalmask',
                        forceRender: true,
                        label: catTabLabel(<RocketOutlined />, t('pages.hosts.fields.finalMask'), isMobile),
                        children: (
                          <Form.Item noStyle>
                            <Controller
                              control={methods.control}
                              name="finalMask"
                              render={({ field }) => (
                                <HostFinalMaskForm value={field.value} onChange={field.onChange} />
                              )}
                            />
                          </Form.Item>
                        ),
                      },
                    ]}
                  />
                ),
              },
              {
                key: 'clash',
                forceRender: true,
                label: catTabLabel(<NodeIndexOutlined />, t('pages.hosts.sections.clash'), isMobile),
                children: (
                  <>
                    <FormField name="mihomoIpVersion" label={t('pages.hosts.fields.mihomoIpVersion')}>
                      <Select
                        allowClear
                        options={['dual', 'ipv4', 'ipv6', 'ipv4-prefer', 'ipv6-prefer'].map((v) => ({ value: v, label: v }))}
                      />
                    </FormField>
                    <FormField name="mihomoX25519" label={t('pages.hosts.fields.mihomoX25519')} valueProp="checked">
                      <Switch />
                    </FormField>
                    <FormField name="shuffleHost" label={t('pages.hosts.fields.shuffleHost')} valueProp="checked">
                      <Switch />
                    </FormField>
                  </>
                ),
              },
            ]}
          />
        </Form>
      </FormProvider>
    </Modal>
  );
}
