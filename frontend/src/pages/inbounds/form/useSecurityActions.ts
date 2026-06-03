import type { Dispatch, SetStateAction } from 'react';
import { useTranslation } from 'react-i18next';
import type { FormInstance } from 'antd';
import type { MessageInstance } from 'antd/es/message/interface';

import { HttpUtil, RandomUtil } from '@/utils';
import { getRandomRealityTarget } from '@/models/reality-targets';
import { TlsStreamSettingsSchema } from '@/schemas/protocols/security/tls';
import { RealityStreamSettingsSchema } from '@/schemas/protocols/security/reality';
import type { InboundFormValues } from '@/schemas/forms/inbound-form';

interface UseSecurityActionsArgs {
  form: FormInstance<InboundFormValues>;
  setSaving: Dispatch<SetStateAction<boolean>>;
  messageApi: MessageInstance;
  // Node the inbound is deployed to (null = central panel). "Set Cert from
  // Panel" must read the node's own cert paths for a node-assigned inbound —
  // the central panel's paths don't exist on the node. See issue #4854.
  nodeId: number | null;
}

// Server-side TLS / Reality key + certificate generation handlers for the
// inbound modal's security tab. Each talks to a /panel server endpoint and
// writes the result back into the form. Lifted out of InboundFormModal so
// the modal body stays focused on orchestration.
export function useSecurityActions({ form, setSaving, messageApi, nodeId }: UseSecurityActionsArgs) {
  const { t } = useTranslation();

  const genRealityKeypair = async () => {
    setSaving(true);
    try {
      const msg = await HttpUtil.get('/panel/api/server/getNewX25519Cert');
      if (msg?.success) {
        const obj = msg.obj as { privateKey: string; publicKey: string };
        form.setFieldValue(['streamSettings', 'realitySettings', 'privateKey'], obj.privateKey);
        form.setFieldValue(['streamSettings', 'realitySettings', 'settings', 'publicKey'], obj.publicKey);
      }
    } finally {
      setSaving(false);
    }
  };

  const clearRealityKeypair = () => {
    form.setFieldValue(['streamSettings', 'realitySettings', 'privateKey'], '');
    form.setFieldValue(['streamSettings', 'realitySettings', 'settings', 'publicKey'], '');
  };

  const genMldsa65 = async () => {
    setSaving(true);
    try {
      const msg = await HttpUtil.get('/panel/api/server/getNewmldsa65');
      if (msg?.success) {
        const obj = msg.obj as { seed: string; verify: string };
        form.setFieldValue(['streamSettings', 'realitySettings', 'mldsa65Seed'], obj.seed);
        form.setFieldValue(['streamSettings', 'realitySettings', 'settings', 'mldsa65Verify'], obj.verify);
      }
    } finally {
      setSaving(false);
    }
  };

  const clearMldsa65 = () => {
    form.setFieldValue(['streamSettings', 'realitySettings', 'mldsa65Seed'], '');
    form.setFieldValue(['streamSettings', 'realitySettings', 'settings', 'mldsa65Verify'], '');
  };

  const randomizeRealityTarget = () => {
    const tgt = getRandomRealityTarget() as { target: string; sni: string };
    form.setFieldValue(['streamSettings', 'realitySettings', 'target'], tgt.target);
    form.setFieldValue(
      ['streamSettings', 'realitySettings', 'serverNames'],
      tgt.sni.split(',').map((s) => s.trim()).filter(Boolean),
    );
  };

  const randomizeShortIds = () => {
    form.setFieldValue(
      ['streamSettings', 'realitySettings', 'shortIds'],
      RandomUtil.randomShortIds().split(',').map((s) => s.trim()).filter(Boolean),
    );
  };

  const getNewEchCert = async () => {
    const sni = form.getFieldValue(['streamSettings', 'tlsSettings', 'serverName']);
    setSaving(true);
    try {
      const msg = await HttpUtil.post('/panel/api/server/getNewEchCert', { sni });
      if (msg?.success) {
        const obj = msg.obj as { echServerKeys: string; echConfigList: string };
        form.setFieldValue(['streamSettings', 'tlsSettings', 'echServerKeys'], obj.echServerKeys);
        form.setFieldValue(['streamSettings', 'tlsSettings', 'settings', 'echConfigList'], obj.echConfigList);
      }
    } finally {
      setSaving(false);
    }
  };

  const clearEchCert = () => {
    form.setFieldValue(['streamSettings', 'tlsSettings', 'echServerKeys'], '');
    form.setFieldValue(['streamSettings', 'tlsSettings', 'settings', 'echConfigList'], '');
  };

  const generateRandomPinHash = () => {
    const bytes = new Uint8Array(32);
    crypto.getRandomValues(bytes);
    const hash = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
    const current = (form.getFieldValue(
      ['streamSettings', 'tlsSettings', 'settings', 'pinnedPeerCertSha256'],
    ) as string[] | undefined) ?? [];
    form.setFieldValue(
      ['streamSettings', 'tlsSettings', 'settings', 'pinnedPeerCertSha256'],
      [...current, hash],
    );
  };

  const setCertFromPanel = async (certName: number) => {
    setSaving(true);
    try {
      // Node-assigned inbounds run on the node, so their cert files must be the
      // node's own paths (fetched through the central panel), not this panel's.
      const msg = typeof nodeId === 'number'
        ? await HttpUtil.get(`/panel/api/nodes/webCert/${nodeId}`, undefined, { silent: true })
        : await HttpUtil.post('/panel/setting/all', undefined, { silent: true });
      if (!msg?.success) {
        messageApi.warning(msg?.msg || t('pages.inbounds.setDefaultCertEmpty'));
        return;
      }
      const obj = msg.obj as { webCertFile?: string; webKeyFile?: string };
      if (!obj?.webCertFile && !obj?.webKeyFile) {
        messageApi.warning(t('pages.inbounds.setDefaultCertEmpty'));
        return;
      }
      form.setFieldValue(
        ['streamSettings', 'tlsSettings', 'certificates', certName, 'certificateFile'],
        obj.webCertFile ?? '',
      );
      form.setFieldValue(
        ['streamSettings', 'tlsSettings', 'certificates', certName, 'keyFile'],
        obj.webKeyFile ?? '',
      );
    } finally {
      setSaving(false);
    }
  };

  const clearCertFiles = (certName: number) => {
    form.setFieldValue(
      ['streamSettings', 'tlsSettings', 'certificates', certName, 'certificateFile'],
      '',
    );
    form.setFieldValue(
      ['streamSettings', 'tlsSettings', 'certificates', certName, 'keyFile'],
      '',
    );
  };

  const onSecurityChange = async (next: string) => {
    const current = (form.getFieldValue('streamSettings') as Record<string, unknown>) ?? {};
    const cleaned: Record<string, unknown> = { ...current, security: next };
    delete cleaned.tlsSettings;
    delete cleaned.realitySettings;
    if (next === 'tls') {
      const tls = TlsStreamSettingsSchema.parse({}) as Record<string, unknown>;
      tls.certificates = [{
        useFile: true,
        certificateFile: '',
        keyFile: '',
        certificate: [],
        key: [],
        oneTimeLoading: false,
        usage: 'encipherment',
        buildChain: false,
      }];
      cleaned.tlsSettings = tls;
    }
    if (next === 'reality') {
      const reality = RealityStreamSettingsSchema.parse({}) as Record<string, unknown>;
      const tgt = getRandomRealityTarget() as { target: string; sni: string };
      reality.target = tgt.target;
      reality.serverNames = tgt.sni.split(',').map((s) => s.trim()).filter(Boolean);
      reality.shortIds = RandomUtil.randomShortIds().split(',').map((s) => s.trim()).filter(Boolean);
      cleaned.realitySettings = reality;
    }
    form.setFieldValue('streamSettings', cleaned);
    if (next === 'reality') {
      try {
        const msg = await HttpUtil.get('/panel/api/server/getNewX25519Cert');
        if (msg?.success) {
          const obj = msg.obj as { privateKey: string; publicKey: string };
          form.setFieldValue(['streamSettings', 'realitySettings', 'privateKey'], obj.privateKey);
          form.setFieldValue(['streamSettings', 'realitySettings', 'settings', 'publicKey'], obj.publicKey);
        }
      } catch {
        // best-effort: leave keypair fields empty if server call fails
      }
    }
  };

  return {
    genRealityKeypair,
    clearRealityKeypair,
    genMldsa65,
    clearMldsa65,
    randomizeRealityTarget,
    randomizeShortIds,
    getNewEchCert,
    clearEchCert,
    generateRandomPinHash,
    setCertFromPanel,
    clearCertFiles,
    onSecurityChange,
  };
}
