import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Collapse, Modal } from 'antd';
import type { CollapseProps } from 'antd';

import { Protocols } from '@/schemas/primitives';
import {
  genAllLinks,
  genWireguardConfigs,
  genWireguardLinks,
  isPostQuantumLink,
  preferPublicHost,
} from '@/lib/xray/inbound-link';
import { inboundFromDb, type DbInboundLike } from '@/lib/xray/inbound-from-db';
import QrPanel from './QrPanel';
import type { SubSettings } from '../useInbounds';

interface ClientSetting {
  email?: string;
  subId?: string;
  [k: string]: unknown;
}

interface QrCodeModalProps {
  open: boolean;
  onClose: () => void;
  dbInbound: (DbInboundLike & { remark?: string }) | null;
  client?: ClientSetting | null;
  remarkModel?: string;
  nodeAddress?: string;
  subSettings?: SubSettings;
}

interface QrItem {
  key: string;
  header: string;
  value: string;
  downloadName?: string;
}

export default function QrCodeModal({
  open,
  onClose,
  dbInbound,
  client = null,
  remarkModel = '-io',
  nodeAddress = '',
  subSettings,
}: QrCodeModalProps) {
  const { t } = useTranslation();
  const [links, setLinks] = useState<{ remark?: string; link: string }[]>([]);
  const [wireguardConfigs, setWireguardConfigs] = useState<string[]>([]);
  const [wireguardLinks, setWireguardLinks] = useState<string[]>([]);
  const [subLink, setSubLink] = useState('');
  const [subJsonLink, setSubJsonLink] = useState('');
  const [activeKey, setActiveKey] = useState<string[]>([]);

  useEffect(() => {
    if (!open || !dbInbound) return;
    const inbound = inboundFromDb(dbInbound);
    const fallbackHostname = preferPublicHost(window.location.hostname, subSettings?.publicHost ?? '');
    if (inbound.protocol === Protocols.WIREGUARD) {
      const peerRemark = client?.email
        ? `${dbInbound.remark}-${client.email}`
        : dbInbound.remark || '';
      setWireguardConfigs(
        genWireguardConfigs({
          inbound,
          remark: peerRemark,
          remarkModel: '-io',
          hostOverride: nodeAddress,
          fallbackHostname,
        }).split('\r\n'),
      );
      setWireguardLinks(
        genWireguardLinks({
          inbound,
          remark: peerRemark,
          remarkModel: '-io',
          hostOverride: nodeAddress,
          fallbackHostname,
        }).split('\r\n'),
      );
      setLinks([]);
    } else {
      setLinks(
        genAllLinks({
          inbound,
          remark: dbInbound.remark || '',
          remarkModel,
          client: client ?? {},
          hostOverride: nodeAddress,
          fallbackHostname,
        }),
      );
      setWireguardConfigs([]);
      setWireguardLinks([]);
    }

    const subId = client?.subId;
    let nextSub = '';
    let nextSubJson = '';
    if (subSettings?.enable && subId) {
      nextSub = (subSettings.subURI || '') + subId;
      nextSubJson = subSettings.subJsonEnable ? (subSettings.subJsonURI || '') + subId : '';
    }
    setSubLink(nextSub);
    setSubJsonLink(nextSubJson);
  }, [open, dbInbound, client, remarkModel, nodeAddress, subSettings]);

  const qrItems = useMemo<QrItem[]>(() => {
    const items: QrItem[] = [];
    if (subLink) {
      items.push({ key: 'sub', header: t('subscription.title'), value: subLink });
    }
    if (subJsonLink) {
      items.push({ key: 'sub-json', header: `${t('subscription.title')} (JSON)`, value: subJsonLink });
    }
    links.forEach((link, idx) => {
      items.push({ key: `l${idx}`, header: link.remark || `Link ${idx + 1}`, value: link.link });
    });
    wireguardConfigs.forEach((cfg, idx) => {
      items.push({
        key: `wc${idx}`,
        header: `Peer ${idx + 1} config`,
        value: cfg,
        downloadName: `peer-${idx + 1}.conf`,
      });
      if (wireguardLinks[idx]) {
        items.push({ key: `wl${idx}`, header: `Peer ${idx + 1} link`, value: wireguardLinks[idx] });
      }
    });
    return items;
  }, [subLink, subJsonLink, links, wireguardConfigs, wireguardLinks, t]);

  const collapseItems: CollapseProps['items'] = useMemo(
    () => qrItems.map((item) => ({
      key: item.key,
      label: item.header,
      children: (
        <QrPanel
          value={item.value}
          remark={item.header}
          downloadName={item.downloadName || ''}
          showQr={!isPostQuantumLink(item.value)}
        />
      ),
    })),
    [qrItems],
  );

  useEffect(() => {
    if (!open) {
      setActiveKey([]);
      return;
    }
    setActiveKey(qrItems.length > 0 ? [qrItems[0].key] : []);
  }, [open, qrItems]);

  return (
    <Modal open={open} onCancel={onClose} title={t('qrCode')} footer={null} width={420} destroyOnHidden>
      {dbInbound && collapseItems && collapseItems.length > 0 && (
        <Collapse
          ghost
          activeKey={activeKey}
          onChange={(keys) => setActiveKey(typeof keys === 'string' ? [keys] : (keys as string[]))}
          items={collapseItems}
        />
      )}
    </Modal>
  );
}
