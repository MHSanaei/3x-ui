import { useTranslation } from 'react-i18next';
import { Form, Input, Select, Switch } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { HeaderMapEditor } from '@/components/form';
import { FormField } from '@/components/form/rhf';
import { MASQUE_DEFAULT_PATH } from '@/schemas/protocols/stream/masque';

const BASE = ['streamSettings', 'masqueSettings'] as const;

// WARP authenticates with its enrolled key instead of Basic auth, and xray-core
// refuses user/pass next to warp; host and path then default to Cloudflare's endpoint.
export default function MasqueForm() {
  const { t } = useTranslation();
  const { control, setValue } = useFormContext();
  const warp = useWatch({ control, name: 'streamSettings.masqueSettings.warp' });
  const warpOn = warp != null;

  return (
    <>
      <FormField label={t('host')} name={[...BASE, 'host']}>
        <Input placeholder={warpOn ? 'cloudflareaccess.com' : undefined} />
      </FormField>
      <FormField label={t('path')} name={[...BASE, 'path']}>
        <Input placeholder={warpOn ? '/' : MASQUE_DEFAULT_PATH} />
      </FormField>
      <Form.Item label="WARP" tooltip={t('pages.xray.outboundForm.masqueWarpDesc')}>
        <Switch
          checked={warpOn}
          onChange={(on) => {
            setValue(
              'streamSettings.masqueSettings.warp',
              on ? { privateKey: '', publicKey: '', address: [] } : undefined,
            );
            if (on) {
              setValue('streamSettings.masqueSettings.user', undefined);
              setValue('streamSettings.masqueSettings.pass', undefined);
            }
          }}
        />
      </Form.Item>
      {warpOn ? (
        <>
          <FormField
            label={t('pages.xray.warp.privateKey')}
            name={[...BASE, 'warp', 'privateKey']}
            required
          >
            <Input.TextArea
              autoSize={{ minRows: 2, maxRows: 6 }}
              placeholder="-----BEGIN PRIVATE KEY-----"
            />
          </FormField>
          <FormField
            label={t('pages.xray.outboundForm.masqueWarpPublicKey')}
            name={[...BASE, 'warp', 'publicKey']}
            required
          >
            <Input.TextArea
              autoSize={{ minRows: 2, maxRows: 6 }}
              placeholder="-----BEGIN PUBLIC KEY-----"
            />
          </FormField>
          <FormField
            label={t('pages.xray.outboundForm.masqueWarpAddress')}
            name={[...BASE, 'warp', 'address']}
            required
          >
            <Select
              mode="tags"
              tokenSeparators={[',', ' ']}
              open={false}
              placeholder="172.16.0.2, 2606:4700:110:8a36::2"
            />
          </FormField>
        </>
      ) : (
        <>
          <FormField label={t('username')} name={[...BASE, 'user']}>
            <Input />
          </FormField>
          <FormField label={t('password')} name={[...BASE, 'pass']}>
            <Input.Password />
          </FormField>
        </>
      )}
      <FormField label={t('pages.inbounds.form.headers')} name={[...BASE, 'headers']}>
        <HeaderMapEditor mode="v1" />
      </FormField>
    </>
  );
}
