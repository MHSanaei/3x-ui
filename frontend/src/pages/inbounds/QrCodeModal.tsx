import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Collapse, Modal } from 'antd';
import type { CollapseProps } from 'antd';

import { Protocols } from '@/models/inbound.js';
import QrPanel from './QrPanel';
import type { SubSettings } from './useInbounds';

interface ClientSetting {
  email?: string;
  subId?: string;
  [k: string]: unknown;
}

interface DBInboundLike {
  remark?: string;
  toInbound: () => InboundLike;
}

interface InboundLike {
  protocol: string;
  genWireguardConfigs: (remark: string, model: string, host: string) => string;
  genWireguardLinks: (remark: string, model: string, host: string) => string;
  genAllLinks: (remark: string, model: string, client: ClientSetting | null, host: string) => { remark?: string; link: string }[];
}

interface QrCodeModalProps {
  open: boolean;
  onClose: () => void;
  dbInbound: DBInboundLike | null;
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
  remarkModel = '-ieo',
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
    const inbound = dbInbound.toInbound();
    if (inbound.protocol === Protocols.WIREGUARD) {
      const peerRemark = client?.email
        ? `${dbInbound.remark}-${client.email}`
        : dbInbound.remark || '';
      setWireguardConfigs(inbound.genWireguardConfigs(peerRemark, '-ieo', nodeAddress).split('\r\n'));
      setWireguardLinks(inbound.genWireguardLinks(peerRemark, '-ieo', nodeAddress).split('\r\n'));
      setLinks([]);
    } else {
      setLinks(inbound.genAllLinks(dbInbound.remark || '', remarkModel, client, nodeAddress) as { remark?: string; link: string }[]);
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
          showQr={!item.value.includes('mldsa65') && !item.value.includes('ML-KEM-768')}
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
