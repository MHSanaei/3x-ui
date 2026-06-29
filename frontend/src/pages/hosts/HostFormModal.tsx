import { useEffect, useMemo } from 'react';
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

import type { HostRecord } from '@/api/queries/useHostsQuery';
import type { HostFormValues } from '@/schemas/api/host';
import type { InboundOption } from '@/schemas/client';
import { ALPN_OPTION, UTLS_FINGERPRINT } from '@/schemas/primitives';
import { useNodesQuery } from '@/api/queries/useNodesQuery';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from '@/pages/settings/catTabLabel';
import { HostFinalMaskForm, HostMuxForm, HostSockoptForm } from './json-forms';

// inboundId is optional in the form so a new host starts unselected (the Select
// shows its placeholder instead of 0); the required rule enforces it on submit.
type FormShape = Omit<HostFormValues, 'isDisabled' | 'inboundId'> & { enable: boolean; inboundId?: number };

interface HostFormModalProps {
  open: boolean;
  mode: 'add' | 'edit';
  host: HostRecord | null;
  inboundOptions: InboundOption[];
  save: (payload: Partial<HostFormValues>) => Promise<{ success?: boolean; msg?: string } | undefined>;
  onOpenChange: (open: boolean) => void;
}

const asString = (v: unknown): string => (typeof v === 'string' ? v : '');

function defaultsFor(host: HostRecord | null): FormShape {
  return {
    inboundId: host?.inboundId,
    sortOrder: host?.sortOrder ?? 0,
    remark: host?.remark ?? '',
    serverDescription: host?.serverDescription ?? '',
    enable: host ? !host.isDisabled : true,
    isHidden: host?.isHidden ?? false,
    tags: host?.tags ?? [],
    address: host?.address ?? '',
    port: host?.port ?? 0,
    security: (host?.security as HostFormValues['security']) ?? 'same',
    sni: host?.sni ?? '',
    hostHeader: host?.hostHeader ?? '',
    path: host?.path ?? '',
    alpn: (host?.alpn as HostFormValues['alpn']) ?? [],
    fingerprint: host?.fingerprint as HostFormValues['fingerprint'],
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
    excludeFromSubTypes: (host?.excludeFromSubTypes as HostFormValues['excludeFromSubTypes']) ?? [],
    nodeGuids: host?.nodeGuids ?? [],
    mihomoIpVersion: host?.mihomoIpVersion as HostFormValues['mihomoIpVersion'],
    mihomoX25519: host?.mihomoX25519 ?? false,
    shuffleHost: host?.shuffleHost ?? false,
  };
}

export default function HostFormModal({ open, mode, host, inboundOptions, save, onOpenChange }: HostFormModalProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const [form] = Form.useForm<FormShape>();

  // Drive conditional field visibility off the selected security, like the
  // legacy externalProxy form: same/none inherit fully and hide every TLS/cert
  // field; reality shows only the reality-relevant subset (its keys are
  // inherited from the inbound); tls shows the full TLS override set.
  const security = (Form.useWatch('security', form) ?? 'same') as string;
  const showTls = security === 'tls' || security === 'reality';
  const showTlsExtras = security === 'tls';

  useEffect(() => {
    if (open) form.setFieldsValue(defaultsFor(host));
  }, [open, host, form]);

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

  const onOk = async () => {
    let values: FormShape;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    const { enable, ...rest } = values;
    const payload: Partial<HostFormValues> = { ...rest, isDisabled: !enable };
    const res = await save(payload);
    if (res?.success) {
      message.success(t(mode === 'add' ? 'pages.hosts.toasts.add' : 'pages.hosts.toasts.update'));
      onOpenChange(false);
    } else if (res?.msg) {
      message.error(res.msg);
    }
  };

  return (
    <Modal
      open={open}
      title={t(mode === 'add' ? 'pages.hosts.addHost' : 'pages.hosts.editHost')}
      onOk={onOk}
      onCancel={() => onOpenChange(false)}
      okText={t('save')}
      cancelText={t('cancel')}
      destroyOnHidden
      width={isMobile ? '95vw' : 760}
      styles={{ body: { maxHeight: '70vh', overflowY: 'auto', overflowX: 'hidden' } }}
    >
      <Form
        form={form}
        colon={false}
        labelCol={{ sm: { span: 8 } }}
        wrapperCol={{ sm: { span: 14 } }}
        labelWrap
        initialValues={defaultsFor(host)}
        preserve={false}
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
                  <Form.Item name="remark" label={t('pages.hosts.fields.remark')} tooltip={t('pages.hosts.hints.remark')} rules={[{ required: true, max: 256 }]}>
                    <Input maxLength={256} />
                  </Form.Item>
                  <Form.Item name="serverDescription" label={t('pages.hosts.fields.serverDescription')} tooltip={t('pages.hosts.hints.serverDescription')}>
                    <Input maxLength={64} />
                  </Form.Item>
                  <Form.Item name="inboundId" label={t('pages.hosts.fields.inbound')} rules={[{ required: true }]}>
                    <Select
                      options={inboundSelectOptions}
                      showSearch
                      optionFilterProp="label"
                      disabled={mode === 'edit'}
                      placeholder={t('pages.hosts.selectInbound')}
                    />
                  </Form.Item>
                  <Form.Item name="address" label={t('pages.hosts.fields.address')} tooltip={t('pages.hosts.hints.address')}>
                    <Input placeholder="cdn.example.com" />
                  </Form.Item>
                  <Form.Item name="port" label={t('pages.hosts.fields.port')} tooltip={t('pages.hosts.hints.port')}>
                    <InputNumber min={0} max={65535} />
                  </Form.Item>
                  <Form.Item name="tags" label={t('pages.hosts.fields.tags')} tooltip={t('pages.hosts.hints.tags')}>
                    <Select mode="tags" allowClear tokenSeparators={[',']} />
                  </Form.Item>
                  <Form.Item name="nodeGuids" label={t('pages.hosts.fields.nodeGuids')} tooltip={t('pages.hosts.hints.nodeGuids')}>
                    <Select mode="multiple" allowClear options={nodeSelectOptions} optionFilterProp="label" />
                  </Form.Item>
                  <Form.Item name="enable" label={t('pages.hosts.fields.enable')} valuePropName="checked">
                    <Switch />
                  </Form.Item>
                </>
              ),
            },
            {
              key: 'security',
              forceRender: true,
              label: catTabLabel(<SafetyCertificateOutlined />, t('pages.hosts.sections.security'), isMobile),
              children: (
                <>
                  <Form.Item name="security" label={t('pages.hosts.fields.security')}>
                    <Select
                      options={['same', 'tls', 'none', 'reality'].map((v) => ({ value: v, label: v }))}
                    />
                  </Form.Item>
                  {showTls && (
                    <>
                      <Form.Item name="sni" label={t('pages.hosts.fields.sni')}>
                        <Input />
                      </Form.Item>
                      <Form.Item name="overrideSniFromAddress" label={t('pages.hosts.fields.overrideSniFromAddress')} valuePropName="checked">
                        <Switch />
                      </Form.Item>
                      <Form.Item name="keepSniBlank" label={t('pages.hosts.fields.keepSniBlank')} valuePropName="checked">
                        <Switch />
                      </Form.Item>
                      <Form.Item name="fingerprint" label={t('pages.hosts.fields.fingerprint')}>
                        <Select allowClear options={fpOptions} />
                      </Form.Item>
                    </>
                  )}
                  {showTlsExtras && (
                    <>
                      <Form.Item name="alpn" label={t('pages.hosts.fields.alpn')}>
                        <Select mode="multiple" allowClear options={alpnOptions} />
                      </Form.Item>
                      <Form.Item name="pinnedPeerCertSha256" label={t('pages.hosts.fields.pins')}>
                        <Select mode="tags" allowClear tokenSeparators={[',']} />
                      </Form.Item>
                      <Form.Item name="verifyPeerCertByName" label={t('pages.hosts.fields.verifyPeerCertByName')} tooltip={t('pages.inbounds.form.verifyPeerCertByNameTip')}>
                        <Input placeholder="example.com" />
                      </Form.Item>
                      <Form.Item name="allowInsecure" label={t('pages.hosts.fields.allowInsecure')} tooltip={t('pages.hosts.hints.allowInsecure')} valuePropName="checked">
                        <Switch />
                      </Form.Item>
                      <Form.Item name="echConfigList" label={t('pages.hosts.fields.echConfigList')}>
                        <Input.TextArea rows={2} />
                      </Form.Item>
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
                          <Form.Item name="hostHeader" label={t('pages.hosts.fields.hostHeader')}>
                            <Input />
                          </Form.Item>
                          <Form.Item name="path" label={t('pages.hosts.fields.path')}>
                            <Input />
                          </Form.Item>
                          <Form.Item name="vlessRoute" label={t('pages.hosts.fields.vlessRoute')} tooltip={t('pages.hosts.hints.vlessRoute')}>
                            <Input placeholder="443" />
                          </Form.Item>
                          <Form.Item name="excludeFromSubTypes" label={t('pages.hosts.fields.excludeFromSubTypes')}>
                            <Select
                              mode="multiple"
                              allowClear
                              options={['raw', 'json', 'clash'].map((v) => ({ value: v, label: v }))}
                            />
                          </Form.Item>
                        </>
                      ),
                    },
                    {
                      key: 'adv-mux',
                      forceRender: true,
                      label: catTabLabel(<PartitionOutlined />, t('pages.hosts.fields.muxParams'), isMobile),
                      children: (
                        <Form.Item name="muxParams" noStyle>
                          <HostMuxForm />
                        </Form.Item>
                      ),
                    },
                    {
                      key: 'adv-sockopt',
                      forceRender: true,
                      label: catTabLabel(<DeploymentUnitOutlined />, t('pages.hosts.fields.sockoptParams'), isMobile),
                      children: (
                        <Form.Item name="sockoptParams" noStyle>
                          <HostSockoptForm />
                        </Form.Item>
                      ),
                    },
                    {
                      key: 'adv-finalmask',
                      forceRender: true,
                      label: catTabLabel(<RocketOutlined />, t('pages.hosts.fields.finalMask'), isMobile),
                      children: (
                        <Form.Item name="finalMask" noStyle>
                          <HostFinalMaskForm />
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
                  <Form.Item name="mihomoIpVersion" label={t('pages.hosts.fields.mihomoIpVersion')}>
                    <Select
                      allowClear
                      options={['dual', 'ipv4', 'ipv6', 'ipv4-prefer', 'ipv6-prefer'].map((v) => ({ value: v, label: v }))}
                    />
                  </Form.Item>
                  <Form.Item name="mihomoX25519" label={t('pages.hosts.fields.mihomoX25519')} valuePropName="checked">
                    <Switch />
                  </Form.Item>
                  <Form.Item name="shuffleHost" label={t('pages.hosts.fields.shuffleHost')} valuePropName="checked">
                    <Switch />
                  </Form.Item>
                </>
              ),
            },
          ]}
        />
      </Form>
    </Modal>
  );
}
