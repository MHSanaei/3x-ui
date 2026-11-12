import { Select, Spin } from 'antd';

declare const loading: boolean;
declare const rest: Record<string, unknown>;
declare function tagRender(): null;
declare function Other(props: { mode: string; children?: unknown }): null;

export const QuietPage = () => <Spin spinning={loading} aria-live="off"><span>42%</span></Spin>;
export const Indicator = () => <Spin size="large" />;
export const Single = () => <Select aria-label="Protocol" />;
export const Tags = () => <Select mode="tags" aria-label="SNI" tagRender={tagRender} />;
export const Forwarded = () => <Select mode="tags" aria-label="SNI" {...rest} />;
export const NotAntd = () => <Other mode="tags"><span>x</span></Other>;
