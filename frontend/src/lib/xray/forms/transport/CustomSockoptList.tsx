import i18next from 'i18next';
import { Button, Divider, Form, Input, Select } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { NamePath } from 'antd/es/form/interface';

import { activateOnKey } from '@/utils/a11y';

// Editor for sockopt.customSockopt — a list of raw setsockopt() options. Each
// entry is rendered as a titled group of labeled fields (system / level / opt /
// type / value) instead of one cramped inline row, so it reads like the rest of
// the sockopt form. Shared by the inbound and outbound (and host) sockopt forms.
// Ref: https://xtls.github.io/config/transports/sockopt.html#sockoptobject

const SYSTEM_OPTIONS = [
  { value: 'linux', label: 'linux' },
  { value: 'windows', label: 'windows' },
  { value: 'darwin', label: 'darwin' },
];

const TYPE_OPTIONS = [
  { value: 'int', label: 'int' },
  { value: 'str', label: 'str' },
];

interface CustomSockoptListProps {
  name?: NamePath;
}

export default function CustomSockoptList({
  name = ['streamSettings', 'sockopt', 'customSockopt'],
}: CustomSockoptListProps) {
  const { t } = useTranslation();
  return (
    <Form.List name={name}>
      {(fields, { add, remove }) => (
        <>
          <Form.Item label={t('pages.inbounds.form.customSockopt')}>
            <Button
              type="dashed"
              size="small"
              icon={<PlusOutlined />}
              onClick={() => add({ type: 'int', level: '6', opt: '', value: '' })}
            >
              {t('pages.inbounds.form.addCustomOption')}
            </Button>
          </Form.Item>
          {fields.map((field, idx) => (
            <div key={field.key}>
              <Divider plain style={{ margin: '4px 0 8px' }}>
                {t('pages.inbounds.form.customSockopt')} {idx + 1}
                <DeleteOutlined
                  className="danger-icon"
                  style={{ marginInlineStart: 8 }}
                  role="button"
                  tabIndex={0}
                  aria-label={t('remove')}
                  onClick={() => remove(field.name)}
                  onKeyDown={activateOnKey(() => remove(field.name))}
                />
              </Divider>
              <Form.Item
                label={i18next.t('pages.settings.eventGroupSystem')}
                name={[field.name, 'system']}
              >
                <Select placeholder={i18next.t('all')} allowClear options={SYSTEM_OPTIONS} />
              </Form.Item>
              <Form.Item label={i18next.t('ui.level')} name={[field.name, 'level']}>
                <Input placeholder="6 (SOL_TCP)" />
              </Form.Item>
              <Form.Item label={i18next.t('ui.opt')} name={[field.name, 'opt']}>
                <Input placeholder={i18next.t('ui.decimalEG19')} />
              </Form.Item>
              <Form.Item label={i18next.t('pages.inbounds.form.type')} name={[field.name, 'type']}>
                <Select options={TYPE_OPTIONS} />
              </Form.Item>
              <Form.Item
                label={i18next.t('pages.xray.ruleForm.value')}
                name={[field.name, 'value']}
              >
                <Input placeholder={i18next.t('pages.xray.ruleForm.value')} />
              </Form.Item>
            </div>
          ))}
        </>
      )}
    </Form.List>
  );
}
