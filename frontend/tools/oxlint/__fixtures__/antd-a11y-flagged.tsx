import { Select, Spin } from 'antd';

declare const loading: boolean;

export const LivePage = () => <Spin spinning={loading}><span>42%</span></Spin>;
export const BareTags = () => <Select mode="tags" aria-label="SNI" />;
export const HiddenTags = () => <Select mode="multiple" aria-label="Inbounds" tagRender={() => null} maxTagCount="responsive" />;
