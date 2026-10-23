import { PlusOutlined, WarningOutlined } from '@ant-design/icons';
import { Button } from 'antd';

declare function CustomOutlined(): null;

export const InButton = () => <Button icon={<PlusOutlined />}>Add</Button>;
export const Standalone = () => <WarningOutlined style={{ color: 'red' }} />;
export const NotAnAntdIcon = () => <CustomOutlined />;
