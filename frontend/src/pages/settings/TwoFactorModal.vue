<script setup>
import { nextTick, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { message } from 'ant-design-vue';
import * as OTPAuth from 'otpauth';
import QRious from 'qrious';

import { ClipboardManager } from '@/utils';

const { t } = useI18n();

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

// Byte-mode capacities (level L) for QR versions 1..40 — used to pick
// the matrix width up front so the canvas size is an exact multiple of
// pixelSize. Without this, QRious renders at floor(size/matrix) and
// centers, leaving a white margin around the QR.
const QR_L_BYTE_CAPACITY = [
  17, 32, 53, 78, 106, 134, 154, 192, 230, 271,
  321, 367, 425, 458, 520, 586, 644, 718, 792, 858,
  929, 1003, 1091, 1171, 1273, 1367, 1465, 1528, 1628, 1732,
  1840, 1952, 2068, 2188, 2303, 2431, 2563, 2699, 2809, 2953,
];

function pickQrMatrixWidth(value) {
  const byteLen = new TextEncoder().encode(value).length;
  for (let i = 0; i < QR_L_BYTE_CAPACITY.length; i++) {
    if (byteLen <= QR_L_BYTE_CAPACITY[i]) return 17 + 4 * (i + 1);
  }
  return 17 + 4 * 40;
}

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
  const value = totp.toString();
  const matrixWidth = pickQrMatrixWidth(value);
  const pixelSize = Math.max(1, Math.floor(200 / matrixWidth));
  const exactSize = matrixWidth * pixelSize;
  new QRious({
    element: qrCanvas.value,
    size: exactSize,
    value,
    background: 'white',
    backgroundAlpha: 1,
    foreground: 'black',
    padding: 0,
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
        <div class="qr-bg">
          <canvas ref="qrCanvas" class="qr-cv" @click="copyToken" />
        </div>
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
  /* Drawing buffer is matrix-snapped (smaller than display size); scale
   * up crisply so the QR fills the box without blurring. */
  image-rendering: pixelated;
  image-rendering: crisp-edges;
}

.qr-token {
  font-size: 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  word-break: break-all;
  text-align: center;
}
</style>
