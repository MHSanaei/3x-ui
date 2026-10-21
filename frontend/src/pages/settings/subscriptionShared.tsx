import i18next from 'i18next';
import { Tag } from 'antd';

export const isRemoteRoutingSource = (value: string) => /^https:\/\/\S+$/i.test(value.trim());

export const remoteSourceBadge = (value: string) =>
  isRemoteRoutingSource(value) ? <Tag color="blue">{i18next.t('ui.hTTPSURL')}</Tag> : undefined;
