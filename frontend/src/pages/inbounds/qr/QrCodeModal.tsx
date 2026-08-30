import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Collapse, Modal } from 'antd';
import type { CollapseProps } from 'antd';

import { Protocols } from '@/schemas/primitives';
import {
  genAllLinks,
  genAmneziaWGConfigs,
  genAmneziaWGLinks,
  genWireguardConfigs,
  genWireguardLinks,
  isPostQuantumLink,
  preferPublicHost,
} from '@/lib/xray/inbound-link';
import { inboundFromDb, type DbInboundLike } from '@/lib/xray/inbound-from-db';
import { withMtprotoHostEndpoints } from '@/lib/hosts/host-link';
import type { HostRecord } from '@/schemas/api/host';
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
  dbInbound: (DbInboundLike & { id: number; remark?: string }) | null;
  client?: ClientSetting | null;
  nodeAddress?: string;
  subSettings?: SubSettings;
  hosts?: HostRecord[];
}

interface QrItem {
  key: string;
  header: string;
  value: string;
  downloadName?: string;
  showQr?: boolean;
}

const EMPTY_HOSTS: HostRecord[] = [];

export default function QrCodeModal({
  open,
  onClose,
  dbInbound,
  client = null,
  nodeAddress = '',
  subSettings,
  hosts = EMPTY_HOSTS,
}: QrCodeModalProps) {
  const { t } = useTranslation();
  const [links, setLinks] = useState<{ remark?: string; link: string }[]>([]);
  const [wireguardConfigs, setWireguardConfigs] = useState<string[]>([]);
  const [wireguardLinks, setWireguardLinks] = useState<string[]>([]);
  const [amneziawgConfigs, setAmneziawgConfigs] = useState<string[]>([]);
  const [amneziawgLinks, setAmneziawgLinks] = useState<string[]>([]);
  const [subLink, setSubLink] = useState('');
  const [subJsonLink, setSubJsonLink] = useState('');
  const [activeKey, setActiveKey] = useState<string[]>([]);

  // Building the links is a pure function of the props, so it runs during
  // render; an effect would paint the previous inbound's QR first.
  const [syncedProps, setSyncedProps] = useState<{
    dbInbound: typeof dbInbound;
    client: typeof client;
    nodeAddress: typeof nodeAddress;
    subSettings: typeof subSettings;
    hosts: typeof hosts;
  } | null>(null);
  if (
    open &&
    dbInbound &&
    (syncedProps === null ||
      syncedProps.dbInbound !== dbInbound ||
      syncedProps.client !== client ||
      syncedProps.nodeAddress !== nodeAddress ||
      syncedProps.subSettings !== subSettings ||
      syncedProps.hosts !== hosts)
  ) {
    setSyncedProps({ dbInbound, client, nodeAddress, subSettings, hosts });
    const inbound = withMtprotoHostEndpoints(inboundFromDb(dbInbound), dbInbound.id, hosts);
    const fallbackHostname = preferPublicHost(
      window.location.hostname,
      subSettings?.publicHost ?? '',
    );
    if (inbound.protocol === Protocols.WIREGUARD) {
      const peerRemark = client?.email
        ? `${dbInbound.remark}-${client.email}`
        : dbInbound.remark || '';
      setWireguardConfigs(
        genWireguardConfigs({
          inbound,
          remark: peerRemark,
          hostOverride: nodeAddress,
          fallbackHostname,
        }).split('\r\n'),
      );
      setWireguardLinks(
        genWireguardLinks({
          inbound,
          remark: peerRemark,
          hostOverride: nodeAddress,
          fallbackHostname,
        }).split('\r\n'),
      );
      setAmneziawgConfigs([]);
      setAmneziawgLinks([]);
      setLinks([]);
    } else if (inbound.protocol === Protocols.AMNEZIAWG) {
      const peerRemark = client?.email
        ? `${dbInbound.remark}-${client.email}`
        : dbInbound.remark || '';
      setAmneziawgConfigs(
        genAmneziaWGConfigs({
          inbound,
          remark: peerRemark,
          hostOverride: nodeAddress,
          fallbackHostname,
        }).split('\r\n'),
      );
      setAmneziawgLinks(
        genAmneziaWGLinks({
          inbound,
          remark: peerRemark,
          hostOverride: nodeAddress,
          fallbackHostname,
        }).split('\r\n'),
      );
      setWireguardConfigs([]);
      setWireguardLinks([]);
      setLinks([]);
    } else {
      setLinks(
        genAllLinks({
          inbound,
          remark: dbInbound.remark || '',
          client: client ?? {},
          hostOverride: nodeAddress,
          fallbackHostname,
        }),
      );
      setWireguardConfigs([]);
      setWireguardLinks([]);
      setAmneziawgConfigs([]);
      setAmneziawgLinks([]);
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
  }

  const qrItems = useMemo<QrItem[]>(() => {
    const items: QrItem[] = [];
    if (subLink) {
      items.push({ key: 'sub', header: t('subscription.title'), value: subLink });
    }
    if (subJsonLink) {
      items.push({
        key: 'sub-json',
        header: `${t('subscription.title')} (JSON)`,
        value: subJsonLink,
      });
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
        items.push({
          key: `wl${idx}`,
          header: `Peer ${idx + 1} link`,
          value: wireguardLinks[idx],
          showQr: false,
        });
      }
    });
    amneziawgConfigs.forEach((cfg, idx) => {
      items.push({
        key: `ac${idx}`,
        header: `Peer ${idx + 1} config`,
        value: cfg,
        downloadName: `peer-${idx + 1}.conf`,
      });
      if (amneziawgLinks[idx]) {
        items.push({
          key: `al${idx}`,
          header: `Peer ${idx + 1} link`,
          value: amneziawgLinks[idx],
          showQr: false,
        });
      }
    });
    return items;
  }, [
    subLink,
    subJsonLink,
    links,
    wireguardConfigs,
    wireguardLinks,
    amneziawgConfigs,
    amneziawgLinks,
    t,
  ]);

  const collapseItems: CollapseProps['items'] = useMemo(
    () =>
      qrItems.map((item) => ({
        key: item.key,
        label: item.header,
        children: (
          <QrPanel
            value={item.value}
            remark={item.header}
            downloadName={item.downloadName || ''}
            showQr={item.showQr !== false && !isPostQuantumLink(item.value)}
          />
        ),
      })),
    [qrItems],
  );

  const firstKey = open && qrItems.length > 0 ? qrItems[0].key : null;
  const [syncedFirstKey, setSyncedFirstKey] = useState<string | null>(null);
  if (firstKey !== syncedFirstKey) {
    setSyncedFirstKey(firstKey);
    setActiveKey(firstKey ? [firstKey] : []);
  }

  return (
    <Modal
      open={open}
      onCancel={onClose}
      title={t('qrCode')}
      footer={null}
      width={420}
      destroyOnHidden
    >
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
