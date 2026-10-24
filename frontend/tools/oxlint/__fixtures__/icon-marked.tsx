import { PlusOutlined, WarningOutlined } from '@ant-design/icons';
import { Button } from 'antd';

declare const rest: Record<string, unknown>;

export const Decorative = () => <Button icon={<PlusOutlined aria-hidden />}>Add</Button>;
export const Named = () => <WarningOutlined aria-label="Disk almost full" />;
export const LabelledBy = () => <WarningOutlined aria-labelledby="warning-text" />;
export const Forwarded = () => <WarningOutlined {...rest} />;
