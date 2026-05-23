import { message as staticMessage } from 'antd';
import type { MessageInstance } from 'antd/es/message/interface';

let current: MessageInstance | typeof staticMessage = staticMessage;

export function setMessageInstance(instance: MessageInstance) {
  current = instance;
}

export function getMessage(): MessageInstance | typeof staticMessage {
  return current;
}
