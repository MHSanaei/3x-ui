<script setup>
import { ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { message } from 'ant-design-vue';
import * as OTPAuth from 'otpauth';

import { ClipboardManager } from '@/utils';

const { t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, default: '' },
  description: { type: String, default: '' },
  token: { type: String, default: '' },
  type: { type: String, default: 'set', validator: (v) => ['set', 'confirm'].includes(v) },
});

const emit = defineEmits(['update:open', 'confirm']);

const enteredCode = ref('');
const qrValue = ref('');

let totp = null;

function buildTotp() {
  totp = new OTPAuth.TOTP({
    issuer: '3x-ui',
    label: 'Administrator',
    algorithm: 'SHA1',
    digits: 6,
    period: 30,
    secret: props.token,
  });
  qrValue.value = totp.toString();
}

watch(() => props.open, (next) => {
  if (!next) return;
  enteredCode.value = '';
  totp = null;
  qrValue.value = '';
  if (props.token) {
    buildTotp();
  }
});

function close(success, code = '') {
  emit('confirm', success, code);
  emit('update:open', false);
  enteredCode.value = '';
}

function onOk() {
  if (props.type === 'confirm' && !props.token) {
    close(true, enteredCode.value);
    return;
  }
  if (!totp) return;
  if (totp.generate() === enteredCode.value) {
    close(true);
  } else {
    message.error(t('pages.settings.security.twoFactorModalError'));
  }
}

function onCancel() {
  close(false);
}

async function copyToken() {
  const ok = await ClipboardManager.copyText(props.token);
  if (ok) message.success(t('copied'));
}
</script>

<template>
  <a-modal :open="open" :title="title" :closable="true" @cancel="onCancel">
    <template v-if="type === 'set'">
      <p>{{ t('pages.settings.security.twoFactorModalSteps') }}</p>
      <a-divider />
      <p>{{ t('pages.settings.security.twoFactorModalFirstStep') }}</p>
      <div class="qr-wrap">
        <a-qrcode class="qr-code" :value="qrValue" :size="180" type="svg" :bordered="false"
          color="#000000" bg-color="#ffffff" error-level="L" :title="t('copy')" @click="copyToken" />
        <span class="qr-token">{{ token }}</span>
      </div>
      <a-divider />
      <p>{{ t('pages.settings.security.twoFactorModalSecondStep') }}</p>
      <a-input v-model:value="enteredCode" :style="{ width: '100%' }" />
    </template>

    <template v-else>
      <p>{{ description }}</p>
      <a-input v-model:value="enteredCode" :style="{ width: '100%' }" />
    </template>

    <template #footer>
      <a-button @click="onCancel">{{ t('cancel') }}</a-button>
      <a-button type="primary" :disabled="enteredCode.length < 6" @click="onOk">{{ t('confirm') }}</a-button>
    </template>
  </a-modal>
</template>

<style scoped>
.qr-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.qr-code {
  cursor: pointer;
  padding: 0 !important;
  background: #fff;
  border-radius: 6px;
}

.qr-token {
  font-size: 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  word-break: break-all;
  text-align: center;
}
</style>
