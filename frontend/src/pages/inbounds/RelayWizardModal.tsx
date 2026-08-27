import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Divider,
  Form,
  Input,
  InputNumber,
  Modal,
  Radio,
  Select,
  Space,
  Typography,
  message,
} from 'antd';

import { PRESET_FALLBACK } from '@/lib/xray/inbound-presets';
import {
  RELAY_ENTRY_PRESETS,
  applyRelayToTemplate,
  buildRelayInboundPayload,
  buildRelayRule,
  landingOutboundFromLink,
  landingOutboundFromManual,
  parseLandingEndpoint,
  uniqueOutboundTag,
  type LandingManualInput,
  type LandingProtocol,
  type RelayOutbound,
} from '@/lib/xray/relay';
import { XrayConfigPayloadSchema } from '@/schemas/xray';
import type { DBInbound } from '@/models/dbinbound';
import { HttpUtil, RandomUtil } from '@/utils';

interface RelayWizardModalProps {
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
  dbInbounds: DBInbound[];
}

const LANDING_PROTOCOLS: { value: LandingProtocol; label: string }[] = [
  { value: 'vless', label: 'VLESS' },
  { value: 'vmess', label: 'VMess' },
  { value: 'trojan', label: 'Trojan' },
  { value: 'shadowsocks', label: 'Shadowsocks' },
  { value: 'socks', label: 'SOCKS5' },
  { value: 'http', label: 'HTTP' },
];

const SS_METHODS = [
  '2022-blake3-aes-256-gcm',
  '2022-blake3-aes-128-gcm',
  'aes-256-gcm',
  'aes-128-gcm',
  'chacha20-ietf-poly1305',
];

// Protocols whose landing config can be filled by pasting a share link.
const LINKABLE = new Set<LandingProtocol>(['vless', 'vmess', 'trojan', 'shadowsocks']);

export default function RelayWizardModal({
  open,
  onClose,
  onCreated,
  dbInbounds,
}: RelayWizardModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [busy, setBusy] = useState(false);

  // Entry (the relay inbound users connect to).
  const [entryMode, setEntryMode] = useState<'new' | 'existing'>('new');
  const [existingEntryId, setExistingEntryId] = useState<number | null>(null);
  const [entryPresetId, setEntryPresetId] = useState(RELAY_ENTRY_PRESETS[0]?.id ?? '');
  const [remark, setRemark] = useState('');
  const [port, setPort] = useState<number>(() => RandomUtil.randomInteger(10000, 60000));

  // Landing (where traffic exits).
  const [landingProtocol, setLandingProtocol] = useState<LandingProtocol>('vless');
  const [inputMode, setInputMode] = useState<'link' | 'manual'>('link');
  const [link, setLink] = useState('');
  const [address, setAddress] = useState('');
  const [landingPort, setLandingPort] = useState<number>(443);
  const [uuid, setUuid] = useState('');
  const [password, setPassword] = useState('');
  const [ssMethod, setSsMethod] = useState(SS_METHODS[0]);
  const [user, setUser] = useState('');
  const [pass, setPass] = useState('');
  // Landing reachability test (relay host → landing IP:port).
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{
    ok: boolean;
    delay?: number;
    error?: string;
  } | null>(null);

  const canLink = LINKABLE.has(landingProtocol);
  const effectiveMode = canLink ? inputMode : 'manual';

  const entryPreset = useMemo(
    () => RELAY_ENTRY_PRESETS.find((p) => p.id === entryPresetId) ?? RELAY_ENTRY_PRESETS[0],
    [entryPresetId],
  );
  const localEntryInbounds = useMemo(
    () =>
      dbInbounds.filter(
        (inbound) => inbound.nodeId == null && inbound.id > 0 && inbound.enable && inbound.tag,
      ),
    [dbInbounds],
  );
  const existingEntry = useMemo(
    () => localEntryInbounds.find((inbound) => inbound.id === existingEntryId),
    [existingEntryId, localEntryInbounds],
  );

  // When the user pastes into the landing-address box, try to auto-split a
  // host:port[:user:pass] or user:pass@host:port endpoint into the separate
  // fields. A plain hostname just sets the address. Recognized creds also
  // fill user/pass (for SOCKS/HTTP landings).
  const onLandingAddressChange = (value: string) => {
    const parsed = parseLandingEndpoint(value);
    if (!parsed) {
      setAddress(value);
      return;
    }
    setAddress(parsed.address);
    if (parsed.port) setLandingPort(parsed.port);
    if (parsed.user !== undefined) setUser(parsed.user);
    if (parsed.pass !== undefined) setPass(parsed.pass);
  };

  // Build the landing outbound (with a finalized unique tag) from the current
  // inputs. Returns an error key when the inputs are incomplete/unparseable.
  function buildLanding(existingTags: string[]): { outbound: RelayOutbound } | { error: string } {
    const tag = uniqueOutboundTag(existingTags, `relay-${landingProtocol}`);
    if (effectiveMode === 'link') {
      const ob = landingOutboundFromLink(link, tag);
      if (!ob)
        return {
          error: t('pages.inbounds.relay.badLink', { defaultValue: '落地分享链接无法解析' }),
        };
      return { outbound: ob };
    }
    if (!address.trim())
      return { error: t('pages.inbounds.relay.needAddress', { defaultValue: '请填写落地地址' }) };
    const input: LandingManualInput = {
      protocol: landingProtocol,
      address: address.trim(),
      port: landingPort,
      id: uuid,
      password,
      method: ssMethod,
      user,
      pass,
    };
    return { outbound: landingOutboundFromManual(input, tag) };
  }

  // Probe whether the relay host can actually reach the landing server. Uses
  // the panel's TCP dial-probe (mode=tcp) on the built landing outbound — a
  // delay means relay → landing IP:port is reachable.
  const testLanding = async () => {
    const built = buildLanding([]);
    if ('error' in built) {
      setTestResult({ ok: false, error: built.error });
      return;
    }
    setTesting(true);
    setTestResult(null);
    try {
      const fd = new FormData();
      fd.append('outbound', JSON.stringify(built.outbound));
      fd.append('mode', 'tcp');
      const msg = await HttpUtil.post('/panel/api/xray/testOutbound', fd);
      const obj = msg?.obj as { success?: boolean; delay?: number; error?: string } | undefined;
      if (msg?.success && obj?.success) {
        setTestResult({ ok: true, delay: obj.delay });
      } else {
        setTestResult({
          ok: false,
          error:
            obj?.error ||
            msg?.msg ||
            t('pages.nodes.connectionFailed', { defaultValue: '连接失败' }),
        });
      }
    } catch (e) {
      setTestResult({ ok: false, error: String(e) });
    } finally {
      setTesting(false);
    }
  };

  const create = async () => {
    if (entryMode === 'new' && !entryPreset) return;
    if (entryMode === 'existing' && !existingEntry) {
      messageApi.error(
        t('pages.inbounds.relay.selectExistingEntry', { defaultValue: '请选择一个已有的本机入口' }),
      );
      return;
    }
    setBusy(true);
    try {
      let relayTag = existingEntry?.tag ?? '';
      if (entryMode === 'new') {
        // 1) Reality keys for a newly created entry inbound.
        let priv = '';
        let pub = '';
        const keyMsg = await HttpUtil.get('/panel/api/server/getNewX25519Cert');
        if (keyMsg?.success && keyMsg.obj) {
          const obj = keyMsg.obj as { privateKey: string; publicKey: string };
          priv = obj.privateKey;
          pub = obj.publicKey;
        } else {
          messageApi.error(
            t('pages.inbounds.relay.keyFailed', { defaultValue: '获取 Reality 密钥失败' }),
          );
          return;
        }

        // 2) Create the entry inbound, read back its server-generated tag.
        const payload = buildRelayInboundPayload(entryPreset!, {
          remark:
            remark.trim() || t('pages.inbounds.relay.defaultRemark', { defaultValue: '中转' }),
          port,
          realityPrivateKey: priv,
          realityPublicKey: pub,
        });
        const addMsg = await HttpUtil.post('/panel/api/inbounds/add', payload);
        if (!addMsg?.success) {
          messageApi.error(
            addMsg?.msg ||
              t('pages.inbounds.relay.entryFailed', { defaultValue: '创建中转入站失败' }),
          );
          return;
        }
        relayTag = (addMsg.obj as { tag?: string } | null)?.tag ?? '';
        if (!relayTag) {
          messageApi.error(
            t('pages.inbounds.relay.noTag', { defaultValue: '未能获取中转入站标签' }),
          );
          return;
        }
      }

      // 3) Load the xray template, splice in the landing outbound + routing rule.
      const cfgMsg = await HttpUtil.post('/panel/api/xray/', undefined, { silent: true });
      if (!cfgMsg?.success || typeof cfgMsg.obj !== 'string') {
        messageApi.error(
          t('pages.inbounds.relay.loadCfgFailed', {
            defaultValue: '读取 Xray 配置失败（中转入站已建，请到 Xray 设置手动加路由）',
          }),
        );
        return;
      }
      const parsedCfg = XrayConfigPayloadSchema.safeParse(JSON.parse(cfgMsg.obj));
      if (!parsedCfg.success) {
        messageApi.error(
          t('pages.inbounds.relay.loadCfgFailed', {
            defaultValue: '读取 Xray 配置失败（中转入站已建，请到 Xray 设置手动加路由）',
          }),
        );
        return;
      }
      const cfg = parsedCfg.data;
      const existingTags = (cfg.xraySetting.outbounds ?? [])
        .map((o) => (o as { tag?: string }).tag)
        .filter((x): x is string => typeof x === 'string');

      const built = buildLanding(existingTags);
      if ('error' in built) {
        messageApi.error(built.error);
        return;
      }
      const rule = buildRelayRule(relayTag, built.outbound.tag);
      const nextSetting = applyRelayToTemplate(cfg.xraySetting, built.outbound, rule);

      // 4) Save the template and restart xray to apply.
      const saveMsg = await HttpUtil.post('/panel/api/xray/update', {
        xraySetting: JSON.stringify(nextSetting),
        outboundTestUrl: cfg.outboundTestUrl || 'https://www.google.com/generate_204',
      });
      if (!saveMsg?.success) {
        messageApi.error(
          saveMsg?.msg || t('pages.inbounds.relay.saveFailed', { defaultValue: '保存路由失败' }),
        );
        return;
      }
      await HttpUtil.post('/panel/api/server/restartXrayService');

      messageApi.success(t('pages.inbounds.relay.created', { defaultValue: '中转已创建' }));
      onCreated();
      onClose();
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={t('pages.inbounds.relay.title', { defaultValue: '新建中转' })}
        okText={t('pages.inbounds.relay.create', { defaultValue: '创建中转' })}
        cancelText={t('close', { defaultValue: '关闭' })}
        confirmLoading={busy}
        width={640}
        onOk={create}
        onCancel={onClose}
        destroyOnHidden
      >
        <Alert
          type="info"
          message={t('pages.inbounds.relay.intro', {
            defaultValue: '用户连接「中转入口」，流量经本机转发到「落地出口」后出网。',
          })}
          style={{ marginBottom: 16 }}
        />

        <Divider titlePlacement="start" style={{ marginTop: 0 }}>
          {t('pages.inbounds.relay.entrySection', { defaultValue: '中转入口（用户连这台）' })}
        </Divider>
        <Form colon={false} labelCol={{ span: 6 }} wrapperCol={{ span: 18 }}>
          <Form.Item label={t('pages.inbounds.relay.entryMode', { defaultValue: '入口来源' })}>
            <Radio.Group value={entryMode} onChange={(e) => setEntryMode(e.target.value)}>
              <Radio value="new">
                {t('pages.inbounds.relay.createNewEntry', { defaultValue: '新建入口' })}
              </Radio>
              <Radio value="existing">
                {t('pages.inbounds.relay.useExistingEntry', { defaultValue: '选择已有入口' })}
              </Radio>
            </Radio.Group>
          </Form.Item>

          {entryMode === 'existing' ? (
            <>
              <Form.Item
                label={t('pages.inbounds.relay.existingEntry', { defaultValue: '已有入口' })}
              >
                <Select<number>
                  value={existingEntryId ?? undefined}
                  onChange={(value) => setExistingEntryId(value)}
                  placeholder={t('pages.inbounds.relay.selectExistingEntry', {
                    defaultValue: '请选择一个已有的本机入口',
                  })}
                  options={localEntryInbounds.map((inbound) => ({
                    value: inbound.id,
                    label: `${inbound.remark || '-'} · ${inbound.protocol || '-'} · ${inbound.port}`,
                  }))}
                  style={{ width: '100%' }}
                  showSearch
                  optionFilterProp="label"
                  notFoundContent={t('pages.inbounds.relay.noExistingEntry', {
                    defaultValue: '暂无可用的本机入口',
                  })}
                />
              </Form.Item>
              <Typography.Paragraph
                type="secondary"
                style={{ fontSize: 12, marginInlineStart: '25%' }}
              >
                {t('pages.inbounds.relay.existingEntryHint', {
                  defaultValue:
                    '只显示本机已启用的入口；远程服务器上的入站不能直接作为本机中转入口。',
                })}
              </Typography.Paragraph>
            </>
          ) : (
            <>
              <Form.Item
                label={t('pages.inbounds.relay.entryProtocol', { defaultValue: '入口协议' })}
              >
                <Radio.Group
                  value={entryPresetId}
                  onChange={(e) => setEntryPresetId(e.target.value)}
                  optionType="button"
                  buttonStyle="solid"
                  options={RELAY_ENTRY_PRESETS.map((p) => ({
                    value: p.id,
                    label: t(p.titleKey, { defaultValue: PRESET_FALLBACK[p.id].title }),
                  }))}
                />
              </Form.Item>
              <Form.Item label={t('pages.inbounds.remark', { defaultValue: '备注' })}>
                <Input
                  value={remark}
                  onChange={(e) => setRemark(e.target.value)}
                  placeholder={t('pages.inbounds.relay.defaultRemark', { defaultValue: '中转' })}
                />
              </Form.Item>
              <Form.Item label={t('pages.inbounds.relay.entryPort', { defaultValue: '入口端口' })}>
                <InputNumber
                  min={1}
                  max={65535}
                  value={port}
                  onChange={(v) => setPort(Number(v) || 0)}
                  style={{ width: 160 }}
                />
              </Form.Item>
            </>
          )}
        </Form>

        <Divider titlePlacement="start">
          {t('pages.inbounds.relay.landingSection', { defaultValue: '落地出口（流量从这出网）' })}
        </Divider>
        <Form colon={false} labelCol={{ span: 6 }} wrapperCol={{ span: 18 }}>
          <Form.Item
            label={t('pages.inbounds.relay.landingProtocol', { defaultValue: '落地协议' })}
          >
            <Select
              value={landingProtocol}
              onChange={(v) => setLandingProtocol(v)}
              options={LANDING_PROTOCOLS}
              style={{ width: 220 }}
            />
          </Form.Item>

          {canLink && (
            <Form.Item label={t('pages.inbounds.relay.inputMode', { defaultValue: '填写方式' })}>
              <Radio.Group value={inputMode} onChange={(e) => setInputMode(e.target.value)}>
                <Radio value="link">
                  {t('pages.inbounds.relay.pasteLink', { defaultValue: '粘贴分享链接' })}
                </Radio>
                <Radio value="manual">
                  {t('pages.inbounds.relay.manual', { defaultValue: '手动填写' })}
                </Radio>
              </Radio.Group>
            </Form.Item>
          )}

          {effectiveMode === 'link' ? (
            <Form.Item label={t('pages.inbounds.relay.landingLink', { defaultValue: '落地链接' })}>
              <Input.TextArea
                value={link}
                onChange={(e) => setLink(e.target.value)}
                rows={3}
                placeholder="vless:// / vmess:// / trojan:// / ss://"
              />
            </Form.Item>
          ) : (
            <>
              <Form.Item
                label={t('pages.inbounds.relay.landingAddress', { defaultValue: '落地地址' })}
                extra={t('pages.inbounds.relay.landingAddressHint', {
                  defaultValue: '可直接粘 host:port 或 host:port:用户名:密码，自动拆分',
                })}
              >
                <Input
                  value={address}
                  onChange={(e) => onLandingAddressChange(e.target.value)}
                  placeholder="1.2.3.4  或  1.2.3.4:1080:user:pass"
                />
              </Form.Item>
              <Form.Item
                label={t('pages.inbounds.relay.landingPort', { defaultValue: '落地端口' })}
              >
                <InputNumber
                  min={1}
                  max={65535}
                  value={landingPort}
                  onChange={(v) => setLandingPort(Number(v) || 0)}
                  style={{ width: 160 }}
                />
              </Form.Item>
              {(landingProtocol === 'vless' || landingProtocol === 'vmess') && (
                <Form.Item label="UUID">
                  <Input
                    value={uuid}
                    onChange={(e) => setUuid(e.target.value)}
                    placeholder="客户端 UUID"
                  />
                </Form.Item>
              )}
              {landingProtocol === 'trojan' && (
                <Form.Item
                  label={t('pages.inbounds.relay.passwordLabel', { defaultValue: '密码' })}
                >
                  <Input value={password} onChange={(e) => setPassword(e.target.value)} />
                </Form.Item>
              )}
              {landingProtocol === 'shadowsocks' && (
                <>
                  <Form.Item label={t('pages.inbounds.relay.method', { defaultValue: '加密方式' })}>
                    <Select
                      value={ssMethod}
                      onChange={setSsMethod}
                      options={SS_METHODS.map((m) => ({ value: m, label: m }))}
                      style={{ width: 260 }}
                    />
                  </Form.Item>
                  <Form.Item
                    label={t('pages.inbounds.relay.passwordLabel', { defaultValue: '密码' })}
                  >
                    <Input value={password} onChange={(e) => setPassword(e.target.value)} />
                  </Form.Item>
                </>
              )}
              {(landingProtocol === 'socks' || landingProtocol === 'http') && (
                <>
                  <Form.Item
                    label={t('pages.inbounds.relay.usernameOptional', {
                      defaultValue: '用户名（可选）',
                    })}
                  >
                    <Input value={user} onChange={(e) => setUser(e.target.value)} />
                  </Form.Item>
                  <Form.Item
                    label={t('pages.inbounds.relay.passwordOptional', {
                      defaultValue: '密码（可选）',
                    })}
                  >
                    <Input value={pass} onChange={(e) => setPass(e.target.value)} />
                  </Form.Item>
                </>
              )}
              <Typography.Paragraph
                type="secondary"
                style={{ fontSize: 12, marginInlineStart: '25%' }}
              >
                {t('pages.inbounds.relay.manualHint', {
                  defaultValue:
                    '手动模式建立不带 TLS 的直连；落地是带 TLS/Reality 的节点时请改用「粘贴分享链接」。',
                })}
              </Typography.Paragraph>
            </>
          )}

          <Form.Item label={t('pages.inbounds.relay.testLanding', { defaultValue: '连通测试' })}>
            <Space wrap>
              <Button loading={testing} onClick={testLanding}>
                {t('pages.inbounds.relay.testLandingBtn', { defaultValue: '测试落地连通' })}
              </Button>
              {testResult?.ok && (
                <Typography.Text type="success">
                  {t('pages.nodes.connectionOk', {
                    ms: testResult.delay,
                    defaultValue: `连通正常 ({ms} ms)`,
                  })}
                </Typography.Text>
              )}
              {testResult && !testResult.ok && (
                <Typography.Text type="danger">
                  {t('pages.nodes.connectionFailed', { defaultValue: '连接失败' })}
                  {testResult.error ? `: ${testResult.error}` : ''}
                </Typography.Text>
              )}
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
