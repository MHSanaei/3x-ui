import type { Dispatch, SetStateAction } from 'react';
import { useTranslation } from 'react-i18next';
import type { UseFormReturn } from 'react-hook-form';
import type { MessageInstance } from 'antd/es/message/interface';

import { HttpUtil, RandomUtil } from '@/utils';
import { createTlsSettingsWithDefaultCert } from '@/lib/xray/inbound-tls-defaults';
import { RealityStreamSettingsSchema } from '@/schemas/protocols/security/reality';
import type { InboundFormValues } from '@/schemas/forms/inbound-form';
import type { RealityScanResult } from '@/generated/types';

interface UseSecurityActionsArgs {
  methods: UseFormReturn<InboundFormValues>;
  setSaving: Dispatch<SetStateAction<boolean>>;
  messageApi: MessageInstance;
  /*
   * Node the inbound is deployed to (null = central panel). "Set Cert from
   * Panel" must read the node's own cert paths for a node-assigned inbound —
   * the central panel's paths don't exist on the node. See issue #4854.
   */
  nodeId: number | null;
  setScanResult: Dispatch<SetStateAction<RealityScanResult | null>>;
  setScanning: Dispatch<SetStateAction<boolean>>;
}

/*
 * Server-side TLS / Reality key + certificate generation handlers for the
 * inbound modal's security tab. Each talks to a /panel server endpoint and
 * writes the result back into the form. Lifted out of InboundFormModal so
 * the modal body stays focused on orchestration.
 */
export function useSecurityActions({ methods, setSaving, messageApi, nodeId, setScanResult, setScanning }: UseSecurityActionsArgs) {
  const { t } = useTranslation();
  const setValue = methods.setValue as unknown as (name: string, value: unknown) => void;
  const getValues = methods.getValues as unknown as (name?: string) => unknown;

  const genRealityKeypair = async () => {
    setSaving(true);
    try {
      const msg = await HttpUtil.get('/panel/api/server/getNewX25519Cert');
      if (msg?.success) {
        const obj = msg.obj as { privateKey: string; publicKey: string };
        setValue('streamSettings.realitySettings.privateKey', obj.privateKey);
        setValue('streamSettings.realitySettings.settings.publicKey', obj.publicKey);
      }
    } finally {
      setSaving(false);
    }
  };

  const clearRealityKeypair = () => {
    setValue('streamSettings.realitySettings.privateKey', '');
    setValue('streamSettings.realitySettings.settings.publicKey', '');
  };

  const genMldsa65 = async () => {
    setSaving(true);
    try {
      const msg = await HttpUtil.get('/panel/api/server/getNewmldsa65');
      if (msg?.success) {
        const obj = msg.obj as { seed: string; verify: string };
        setValue('streamSettings.realitySettings.mldsa65Seed', obj.seed);
        setValue('streamSettings.realitySettings.settings.mldsa65Verify', obj.verify);
      }
    } finally {
      setSaving(false);
    }
  };

  const clearMldsa65 = () => {
    setValue('streamSettings.realitySettings.mldsa65Seed', '');
    setValue('streamSettings.realitySettings.settings.mldsa65Verify', '');
  };

  const applyRealityScanResult = (r: RealityScanResult) => {
    setScanResult(r);
    setValue('streamSettings.realitySettings.target', r.target);
    if (r.serverNames?.length) {
      setValue('streamSettings.realitySettings.serverNames', r.serverNames);
    }
  };

  const scanRealityTarget = async () => {
    const target = ((getValues('streamSettings.realitySettings.target') as string | undefined) ?? '').trim();
    if (!target) {
      messageApi.warning(t('pages.inbounds.form.realityTargetRequired'));
      return;
    }
    setScanning(true);
    try {
      const msg = await HttpUtil.post<RealityScanResult>(
        '/panel/api/server/scanRealityTarget',
        { target },
        { silent: true },
      );
      if (!msg?.success || !msg.obj) {
        setScanResult(null);
        messageApi.error(msg?.msg || t('pages.inbounds.toasts.scanRealityTargetError'));
        return;
      }
      const r = msg.obj;
      applyRealityScanResult(r);
      if (r.feasible) {
        messageApi.success(t('pages.inbounds.toasts.scanRealityTargetFeasible'));
      } else {
        messageApi.warning(r.reason || t('pages.inbounds.toasts.scanRealityTargetNotFeasible'));
      }
    } finally {
      setScanning(false);
    }
  };

  const scanRealityCandidates = async (targets?: string): Promise<RealityScanResult[]> => {
    const msg = await HttpUtil.post<RealityScanResult[]>(
      '/panel/api/server/scanRealityTargets',
      targets ? { targets } : {},
      { silent: true },
    );
    if (!msg?.success || !Array.isArray(msg.obj)) {
      messageApi.error(msg?.msg || t('pages.inbounds.toasts.scanRealityTargetError'));
      return [];
    }
    return msg.obj;
  };

  const randomizeShortIds = () => {
    setValue(
      'streamSettings.realitySettings.shortIds',
      RandomUtil.randomShortIds().split(',').map((s) => s.trim()).filter(Boolean),
    );
  };

  const randomizeSpiderX = () => {
    setValue(
      'streamSettings.realitySettings.settings.spiderX',
      `/${RandomUtil.randomSeq(15)}`,
    );
  };

  const getNewEchCert = async () => {
    const sni = getValues('streamSettings.tlsSettings.serverName');
    setSaving(true);
    try {
      const msg = await HttpUtil.post('/panel/api/server/getNewEchCert', { sni });
      if (msg?.success) {
        const obj = msg.obj as { echServerKeys: string; echConfigList: string };
        setValue('streamSettings.tlsSettings.echServerKeys', obj.echServerKeys);
        setValue('streamSettings.tlsSettings.settings.echConfigList', obj.echConfigList);
      }
    } finally {
      setSaving(false);
    }
  };

  const clearEchCert = () => {
    setValue('streamSettings.tlsSettings.echServerKeys', '');
    setValue('streamSettings.tlsSettings.settings.echConfigList', '');
  };

  /*
   * Fill the pinned-cert field from the inbound's own certificate: read the
   * first configured cert (file path or inline content) and ask the server for
   * its hex SHA-256, then merge the hash(es) into pinnedPeerCertSha256.
   */
  const pinFromCert = async () => {
    const certs = (getValues('streamSettings.tlsSettings.certificates') ?? []) as Array<{
      certificateFile?: string;
      certificate?: string[];
    }>;
    const first = certs[0];
    const certFile = first?.certificateFile?.trim() ?? '';
    const certContent = Array.isArray(first?.certificate) ? first.certificate.join('\n').trim() : '';
    if (!certFile && !certContent) {
      messageApi.warning(t('pages.inbounds.setDefaultCertEmpty'));
      return;
    }
    setSaving(true);
    try {
      const msg = await HttpUtil.post('/panel/api/server/getCertHash', { certFile, certContent });
      if (!msg?.success) {
        messageApi.warning(msg?.msg || t('pages.inbounds.setDefaultCertEmpty'));
        return;
      }
      const hashes = (msg.obj as string[] | undefined) ?? [];
      if (hashes.length === 0) return;
      const current = (getValues(
        'streamSettings.tlsSettings.settings.pinnedPeerCertSha256',
      ) as string[] | undefined) ?? [];
      const merged = Array.from(new Set([...current, ...hashes]));
      setValue('streamSettings.tlsSettings.settings.pinnedPeerCertSha256', merged);
    } finally {
      setSaving(false);
    }
  };

  /*
   * Fill the pinned-cert field by pinging the configured SNI: fetches the live
   * remote certificate hash via `xray tls ping`. Useful when the panel doesn't
   * hold the cert file (a CDN front / external endpoint).
   */
  const pinFromRemote = async () => {
    const server = ((getValues('streamSettings.tlsSettings.serverName') as string | undefined) ?? '').trim();
    if (!server) {
      messageApi.warning(t('pages.inbounds.form.pinFromRemoteNoSni'));
      return;
    }
    /*
     * `xray tls ping` defaults to :443, but a self-hosted inbound rarely
     * listens there. Append the inbound's own port (unless the SNI already
     * carries one) so the ping reaches the actual TLS endpoint.
     */
    const port = getValues('port') as number | undefined;
    const target = /:\d+$/.test(server) || !port ? server : `${server}:${port}`;
    setSaving(true);
    try {
      const msg = await HttpUtil.post('/panel/api/server/getRemoteCertHash', { server: target });
      if (!msg?.success) {
        messageApi.warning(msg?.msg || t('pages.inbounds.form.pinFromRemoteFailed'));
        return;
      }
      const hashes = (msg.obj as string[] | undefined) ?? [];
      if (hashes.length === 0) return;
      const current = (getValues(
        'streamSettings.tlsSettings.settings.pinnedPeerCertSha256',
      ) as string[] | undefined) ?? [];
      const merged = Array.from(new Set([...current, ...hashes]));
      setValue('streamSettings.tlsSettings.settings.pinnedPeerCertSha256', merged);
    } finally {
      setSaving(false);
    }
  };

  const setCertFromPanel = async (certName: number) => {
    setSaving(true);
    try {
      /*
       * Node-assigned inbounds run on the node, so their cert files must be the
       * node's own paths (fetched through the central panel), not this panel's.
       */
      const msg = typeof nodeId === 'number'
        ? await HttpUtil.get(`/panel/api/nodes/webCert/${nodeId}`, undefined, { silent: true })
        : await HttpUtil.post('/panel/api/setting/all', undefined, { silent: true });
      if (!msg?.success) {
        messageApi.warning(msg?.msg || t('pages.inbounds.setDefaultCertEmpty'));
        return;
      }
      const obj = msg.obj as { webCertFile?: string; webKeyFile?: string };
      if (!obj?.webCertFile && !obj?.webKeyFile) {
        messageApi.warning(t('pages.inbounds.setDefaultCertEmpty'));
        return;
      }
      setValue(
        `streamSettings.tlsSettings.certificates.${certName}.certificateFile`,
        obj.webCertFile ?? '',
      );
      setValue(
        `streamSettings.tlsSettings.certificates.${certName}.keyFile`,
        obj.webKeyFile ?? '',
      );
    } finally {
      setSaving(false);
    }
  };

  const clearCertFiles = (certName: number) => {
    setValue(
      `streamSettings.tlsSettings.certificates.${certName}.certificateFile`,
      '',
    );
    setValue(
      `streamSettings.tlsSettings.certificates.${certName}.keyFile`,
      '',
    );
  };

  const onSecurityChange = async (next: string) => {
    setScanResult(null);
    const current = (getValues('streamSettings') as Record<string, unknown>) ?? {};
    const cleaned: Record<string, unknown> = { ...current, security: next };
    delete cleaned.tlsSettings;
    delete cleaned.realitySettings;
    if (next === 'tls') {
      cleaned.tlsSettings = createTlsSettingsWithDefaultCert();
    }
    if (next === 'reality') {
      const reality = RealityStreamSettingsSchema.parse({}) as Record<string, unknown>;
      reality.target = '';
      reality.serverNames = [];
      reality.shortIds = RandomUtil.randomShortIds().split(',').map((s) => s.trim()).filter(Boolean);
      cleaned.realitySettings = reality;
    }
    setValue('streamSettings', cleaned);
    if (next === 'reality') {
      randomizeSpiderX();
      try {
        const msg = await HttpUtil.get('/panel/api/server/getNewX25519Cert');
        if (msg?.success) {
          const obj = msg.obj as { privateKey: string; publicKey: string };
          setValue('streamSettings.realitySettings.privateKey', obj.privateKey);
          setValue('streamSettings.realitySettings.settings.publicKey', obj.publicKey);
        }
      } catch {
        /* best-effort: leave keypair fields empty if server call fails */
      }
    }
  };

  return {
    genRealityKeypair,
    clearRealityKeypair,
    genMldsa65,
    clearMldsa65,
    scanRealityTarget,
    scanRealityCandidates,
    applyRealityScanResult,
    randomizeShortIds,
    randomizeSpiderX,
    getNewEchCert,
    clearEchCert,
    pinFromCert,
    pinFromRemote,
    setCertFromPanel,
    clearCertFiles,
    onSecurityChange,
  };
}
