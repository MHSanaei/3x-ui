<script setup>
import { nextTick, ref, watch } from 'vue';
import { message } from 'ant-design-vue';
import * as OTPAuth from 'otpauth';
import QRious from 'qrious';

import { ClipboardManager } from '@/utils';

// Two flavors of this modal:
//   • type='set' shows a QR code + manual key + a 6-digit verifier
//     (used when enabling 2FA the first time);
//   • type='confirm' shows just the 6-digit verifier (used when
//     toggling 2FA off and when changing the admin user/password).
//
// Either way the parent supplies a `confirm(success: boolean)`
// callback — we run it with `true` only if the entered code matches
// the live TOTP value, otherwise `false`.

const props = defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, default: '' },
  description: { type: String, default: '' },
  token: { type: String, default: '' },
  type: { type: String, default: 'set', validator: (v) => ['set', 'confirm'].includes(v) },
});

const emit = defineEmits(['update:open', 'confirm']);

const enteredCode = ref('');
const qrCanvas = ref(null);

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
}

async function paintQr() {
  await nextTick();
  if (!qrCanvas.value || !totp) return;
  // QRious draws into a <canvas>; we don't need a wrapping div.
  // eslint-disable-next-line no-new
  new QRious({
    element: qrCanvas.value,
    size: 200,
    value: totp.toString(),
    background: 'white',
    backgroundAlpha: 0,
    foreground: 'black',
    padding: 2,
    level: 'L',
  });
}

watch(() => props.open, (next) => {
  if (!next) return;
  enteredCode.value = '';
  if (props.token) {
    buildTotp();
    if (props.type === 'set') paintQr();
  }
});

function close(success) {
  emit('confirm', success);
  emit('update:open', false);
  enteredCode.value = '';
}

function onOk() {
  if (!totp) return;
  if (totp.generate() === enteredCode.value) {
    close(true);
  } else {
    message.error('Invalid code — check your authenticator and try again.');
  }
}

function onCancel() {
  close(false);
}

async function copyToken() {
  const ok = await ClipboardManager.copyText(props.token);
  if (ok) message.success('Copied');
}
</script>

<template>
  <a-modal
    :open="open"
    :title="title"
    :closable="true"
    @cancel="onCancel"
  >
    <template v-if="type === 'set'">
      <p>Scan the QR code with your authenticator app, then enter the 6-digit code it shows.</p>
      <a-divider />
      <p>Step 1 — Scan the QR code (click to copy the secret).</p>
      <div class="qr-wrap">
        <div class="qr-bg">
          <canvas ref="qrCanvas" class="qr-cv" @click="copyToken" />
        </div>
        <span class="qr-token">{{ token }}</span>
      </div>
      <a-divider />
      <p>Step 2 — Enter the 6-digit code from your authenticator.</p>
      <a-input v-model:value="enteredCode" :style="{ width: '100%' }" />
    </template>

    <template v-else>
      <p>{{ description }}</p>
      <a-input v-model:value="enteredCode" :style="{ width: '100%' }" />
    </template>

    <template #footer>
      <a-button @click="onCancel">Cancel</a-button>
      <a-button type="primary" :disabled="enteredCode.length < 6" @click="onOk">Confirm</a-button>
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
.qr-bg {
  width: 180px;
  height: 180px;
  background: #fff;
  padding: 4px;
  border-radius: 6px;
}
.qr-cv {
  cursor: pointer;
  width: 100% !important;
  height: 100% !important;
}
.qr-token {
  font-size: 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  word-break: break-all;
  text-align: center;
}
</style>
