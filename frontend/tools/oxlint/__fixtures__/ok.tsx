import { InputNumber } from 'antd';

import { onNumber } from '@/utils/onNumber';

declare function setPort(value: number | null): void;

export const Wrapped = () => <InputNumber onChange={onNumber(setPort)} />;
