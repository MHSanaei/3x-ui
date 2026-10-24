import { Form, Input, Space, Switch } from 'antd';

declare function SettingListItem(props: { title?: string; children?: unknown }): null;

export const TableCell = () => <Switch checked />;

export const LabelOnlyItem = () => (
  <Form.Item label="Email">
    <Input />
  </Form.Item>
);

export const IdWithoutHtmlFor = () => (
  <Form.Item label="Email">
    <Input id="email" />
  </Form.Item>
);

export const HtmlForWithoutId = () => (
  <Form.Item label="Email" htmlFor="email">
    <Input />
  </Form.Item>
);

export const NamedItemNested = () => (
  <Form.Item label="Email" name="email">
    <Space.Compact>
      <Input />
    </Space.Compact>
  </Form.Item>
);

export const SettingNested = () => (
  <SettingListItem title="Enabled">
    <Space>
      <Switch />
    </Space>
  </SettingListItem>
);
