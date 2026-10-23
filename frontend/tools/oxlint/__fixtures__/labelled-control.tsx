import { Form, Input, Space, Switch } from 'antd';

declare function FormField(props: { label?: string; children?: unknown }): null;
declare function SettingListItem(props: { title?: string; control?: unknown; children?: unknown }): null;
declare const rest: Record<string, unknown>;

export const AriaLabel = () => <Switch aria-label="Enabled" />;
export const LabelledBy = () => <Switch aria-labelledby="heading" />;
export const Forwarded = () => <Input {...rest} />;
export const Wrapped = () => (
  // eslint-disable-next-line jsx-a11y/label-has-associated-control -- antd Input is opaque to it
  <label>
    Email <Input />
  </label>
);

export const HtmlForAndId = () => (
  <Form.Item label="Email" htmlFor="email">
    <Space.Compact>
      <Input id="email" />
    </Space.Compact>
  </Form.Item>
);

export const NamedItemDirect = () => (
  <Form.Item label="Email" name="email">
    <Input />
  </Form.Item>
);

export const ViaFormField = () => (
  <FormField label="Email">
    <Input />
  </FormField>
);

export const SettingChild = () => (
  <SettingListItem title="Enabled">
    <Switch />
  </SettingListItem>
);

export const SettingControlProp = () => <SettingListItem title="Enabled" control={<Switch />} />;
