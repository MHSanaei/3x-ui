<script setup>
import { onMounted, reactive, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { Modal, message } from 'ant-design-vue';

import { HttpUtil, RandomUtil } from '@/utils';
import SettingListItem from '@/components/SettingListItem.vue';
import TwoFactorModal from './TwoFactorModal.vue';

const { t } = useI18n();

const props = defineProps({
  allSetting: { type: Object, required: true },
});

// 2FA modal state — both the "set" (enabling) and "confirm" (changing
// password / disabling) flows route through the same component.
const tfa = reactive({
  open: false,
  title: '',
  description: '',
  token: '',
  type: 'set',
  // resolveConfirm is called by the modal's @confirm with the success bool;
  // it then routes the value back to whichever flow opened the modal.
  resolveConfirm: (_success) => { },
});

function openTfa({ title, description = '', token = '', type, onConfirm }) {
  tfa.title = title;
  tfa.description = description;
  tfa.token = token;
  tfa.type = type;
  tfa.resolveConfirm = onConfirm;
  tfa.open = true;
}

function onTfaConfirm(success) {
  tfa.resolveConfirm(success);
}

const user = reactive({
  oldUsername: '',
  oldPassword: '',
  newUsername: '',
  newPassword: '',
});
const updating = ref(false);

async function sendUpdateUser() {
  updating.value = true;
  try {
    const msg = await HttpUtil.post('/panel/setting/updateUser', user);
    if (msg?.success) {
      // Force re-login at the standard logout path; basePath is handled
      // by the Go router so a relative redirect is correct here.
      const basePath = window.__X_UI_BASE_PATH__ || '';
      window.location.replace(`${basePath}logout`);
    }
  } finally {
    updating.value = false;
  }
}

function updateUser() {
  if (props.allSetting.twoFactorEnable) {
    openTfa({
      title: t('pages.settings.security.twoFactorModalChangeCredentialsTitle'),
      description: t('pages.settings.security.twoFactorModalChangeCredentialsStep'),
      token: props.allSetting.twoFactorToken,
      type: 'confirm',
      onConfirm: (ok) => { if (ok) sendUpdateUser(); },
    });
  } else {
    sendUpdateUser();
  }
}

// === API Token =========================================================
// Surfaces the panel's API token so a remote central panel can register
// this instance as a node. Lazy-loaded on tab mount; rotation requires
// confirmation since it invalidates any cached value upstream.
const apiToken = ref('');
const apiTokenLoading = ref(false);
const apiTokenRotating = ref(false);

async function loadApiToken() {
  apiTokenLoading.value = true;
  try {
    const msg = await HttpUtil.get('/panel/setting/getApiToken');
    if (msg?.success) apiToken.value = msg.obj || '';
  } finally {
    apiTokenLoading.value = false;
  }
}

async function copyApiToken() {
  if (!apiToken.value) return;
  try {
    await navigator.clipboard.writeText(apiToken.value);
    message.success(t('copySuccess'));
  } catch (_e) {
    // navigator.clipboard can be undefined on http:// — fall back to
    // a transient input + execCommand path.
    const ta = document.createElement('textarea');
    ta.value = apiToken.value;
    document.body.appendChild(ta);
    ta.select();
    document.execCommand('copy');
    document.body.removeChild(ta);
    message.success(t('copySuccess'));
  }
}

function regenerateApiToken() {
  Modal.confirm({
    title: t('pages.nodes.regenerateConfirm'),
    okText: t('confirm'),
    cancelText: t('cancel'),
    okType: 'danger',
    onOk: async () => {
      apiTokenRotating.value = true;
      try {
        const msg = await HttpUtil.post('/panel/setting/regenerateApiToken');
        if (msg?.success) {
          apiToken.value = msg.obj || '';
          message.success(t('success'));
        }
      } finally {
        apiTokenRotating.value = false;
      }
    },
  });
}

onMounted(loadApiToken);

function toggleTwoFactor() {
  // Switch read-only — the actual flip happens after the modal succeeds.
  if (!props.allSetting.twoFactorEnable) {
    const newToken = RandomUtil.randomBase32String();
    openTfa({
      title: t('pages.settings.security.twoFactorModalSetTitle'),
      token: newToken,
      type: 'set',
      onConfirm: (ok) => {
        if (ok) {
          message.success(t('pages.settings.security.twoFactorModalSetSuccess'));
          props.allSetting.twoFactorToken = newToken;
        }
        props.allSetting.twoFactorEnable = ok;
      },
    });
  } else {
    openTfa({
      title: t('pages.settings.security.twoFactorModalDeleteTitle'),
      description: t('pages.settings.security.twoFactorModalRemoveStep'),
      token: props.allSetting.twoFactorToken,
      type: 'confirm',
      onConfirm: (ok) => {
        if (!ok) return;
        message.success(t('pages.settings.security.twoFactorModalDeleteSuccess'));
        props.allSetting.twoFactorEnable = false;
        props.allSetting.twoFactorToken = '';
      },
    });
  }
}
</script>

<template>
  <a-collapse default-active-key="1">
    <a-collapse-panel key="1" :header="t('pages.settings.security.admin')">
      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.oldUsername') }}</template>
        <template #control>
          <a-input v-model:value="user.oldUsername" autocomplete="username" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.currentPassword') }}</template>
        <template #control>
          <a-input-password v-model:value="user.oldPassword" autocomplete="current-password" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.newUsername') }}</template>
        <template #control>
          <a-input v-model:value="user.newUsername" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.newPassword') }}</template>
        <template #control>
          <a-input-password v-model:value="user.newPassword" autocomplete="new-password" />
        </template>
      </SettingListItem>

      <a-list-item>
        <a-space direction="horizontal" :style="{ padding: '0 20px' }">
          <a-button type="primary" :loading="updating" @click="updateUser">{{ t('confirm') }}</a-button>
        </a-space>
      </a-list-item>
    </a-collapse-panel>

    <a-collapse-panel key="2" :header="t('pages.settings.security.twoFactor')">
      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.security.twoFactorEnable') }}</template>
        <template #description>{{ t('pages.settings.security.twoFactorEnableDesc') }}</template>
        <template #control>
          <a-switch :checked="allSetting.twoFactorEnable" @click="toggleTwoFactor" />
        </template>
      </SettingListItem>
    </a-collapse-panel>

    <a-collapse-panel key="3" :header="t('pages.nodes.apiToken')">
      <SettingListItem paddings="small">
        <template #title>{{ t('pages.nodes.apiToken') }}</template>
        <template #description>{{ t('pages.nodes.apiTokenHint') }}</template>
        <template #control>
          <a-input-password :value="apiToken" readonly :loading="apiTokenLoading" style="min-width: 240px" />
        </template>
      </SettingListItem>
      <a-list-item>
        <a-space direction="horizontal" :style="{ padding: '0 20px' }">
          <a-button :disabled="!apiToken" @click="copyApiToken">{{ t('copy') }}</a-button>
          <a-button danger :loading="apiTokenRotating" @click="regenerateApiToken">
            {{ t('pages.nodes.regenerate') }}
          </a-button>
        </a-space>
      </a-list-item>
    </a-collapse-panel>
  </a-collapse>

  <TwoFactorModal v-model:open="tfa.open" :title="tfa.title" :description="tfa.description" :token="tfa.token"
    :type="tfa.type" @confirm="onTfaConfirm" />
</template>
