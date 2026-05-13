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
      await HttpUtil.post('/logout');
      const basePath = window.X_UI_BASE_PATH || '/';
      window.location.replace(basePath);
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

const apiTokens = ref([]);
const apiTokensLoading = ref(false);
const visibleTokenIds = ref(new Set());
const createOpen = ref(false);
const createName = ref('');
const creating = ref(false);

async function loadApiTokens() {
  apiTokensLoading.value = true;
  try {
    const msg = await HttpUtil.get('/panel/setting/apiTokens');
    if (msg?.success) apiTokens.value = Array.isArray(msg.obj) ? msg.obj : [];
  } finally {
    apiTokensLoading.value = false;
  }
}

function isTokenVisible(id) {
  return visibleTokenIds.value.has(id);
}

function toggleTokenVisibility(id) {
  const next = new Set(visibleTokenIds.value);
  if (next.has(id)) next.delete(id); else next.add(id);
  visibleTokenIds.value = next;
}

async function copyToken(token) {
  if (!token) return;
  try {
    await navigator.clipboard.writeText(token);
    message.success(t('copySuccess'));
  } catch (_e) {
    const ta = document.createElement('textarea');
    ta.value = token;
    document.body.appendChild(ta);
    ta.select();
    document.execCommand('copy');
    document.body.removeChild(ta);
    message.success(t('copySuccess'));
  }
}

function openCreateModal() {
  createName.value = '';
  createOpen.value = true;
}

async function confirmCreateToken() {
  const name = createName.value.trim();
  if (!name) {
    message.error(t('pages.settings.security.apiTokenNameRequired') || 'Name is required');
    return;
  }
  creating.value = true;
  try {
    const msg = await HttpUtil.post('/panel/setting/apiTokens/create', { name });
    if (msg?.success) {
      createOpen.value = false;
      await loadApiTokens();
      if (msg.obj?.id != null) {
        const next = new Set(visibleTokenIds.value);
        next.add(msg.obj.id);
        visibleTokenIds.value = next;
      }
    }
  } finally {
    creating.value = false;
  }
}

function confirmDeleteToken(row) {
  Modal.confirm({
    title: `${t('delete')} "${row.name}"?`,
    content: t('pages.settings.security.apiTokenDeleteWarning')
      || 'Any caller using this token will stop authenticating immediately.',
    okText: t('delete'),
    cancelText: t('cancel'),
    okType: 'danger',
    onOk: async () => {
      const msg = await HttpUtil.post(`/panel/setting/apiTokens/delete/${row.id}`);
      if (msg?.success) await loadApiTokens();
    },
  });
}

async function toggleTokenEnabled(row) {
  const target = !row.enabled;
  const msg = await HttpUtil.post(`/panel/setting/apiTokens/setEnabled/${row.id}`, { enabled: target });
  if (msg?.success) row.enabled = target;
}

function maskToken(token) {
  if (!token) return '';
  return '•'.repeat(Math.min(token.length, 24));
}

function formatTokenDate(ts) {
  if (!ts) return '';
  return new Date(ts * 1000).toLocaleString();
}

onMounted(loadApiTokens);

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
      <div class="api-token-section">
        <div class="api-token-header">
          <p class="api-token-hint">{{ t('pages.nodes.apiTokenHint') }}</p>
          <a-button type="primary" size="small" @click="openCreateModal">
            + {{ t('pages.settings.security.apiTokenNew') || 'New token' }}
          </a-button>
        </div>

        <a-spin :spinning="apiTokensLoading">
          <a-empty v-if="!apiTokens.length && !apiTokensLoading"
            :description="t('pages.settings.security.apiTokenEmpty') || 'No tokens yet'" />

          <div v-for="row in apiTokens" :key="row.id" class="api-token-row" :class="{ disabled: !row.enabled }">
            <div class="api-token-row-head">
              <div class="api-token-name-wrap">
                <span class="api-token-name">{{ row.name }}</span>
                <span class="api-token-created">{{ formatTokenDate(row.createdAt) }}</span>
              </div>
              <div class="api-token-actions">
                <a-switch size="small" :checked="row.enabled" @change="toggleTokenEnabled(row)" />
                <a-button size="small" danger type="text" @click="confirmDeleteToken(row)">
                  {{ t('delete') }}
                </a-button>
              </div>
            </div>
            <div class="api-token-value-wrap">
              <code class="api-token-value">{{ isTokenVisible(row.id) ? row.token : maskToken(row.token) }}</code>
              <a-button size="small" @click="toggleTokenVisibility(row.id)">
                {{ isTokenVisible(row.id)
                  ? (t('pages.settings.security.hide') || 'Hide')
                  : (t('pages.settings.security.show') || 'Show') }}
              </a-button>
              <a-button size="small" @click="copyToken(row.token)">{{ t('copy') }}</a-button>
            </div>
          </div>
        </a-spin>
      </div>
    </a-collapse-panel>
  </a-collapse>

  <a-modal v-model:open="createOpen" :title="t('pages.settings.security.apiTokenNew') || 'New API token'"
    :confirm-loading="creating" :ok-text="t('confirm')" :cancel-text="t('cancel')" @ok="confirmCreateToken">
    <a-form layout="vertical">
      <a-form-item :label="t('pages.settings.security.apiTokenName') || 'Name'" required>
        <a-input v-model:value="createName" maxlength="64"
          :placeholder="t('pages.settings.security.apiTokenNamePlaceholder') || 'e.g. central-panel-a'"
          @keyup.enter="confirmCreateToken" />
      </a-form-item>
    </a-form>
  </a-modal>

  <TwoFactorModal v-model:open="tfa.open" :title="tfa.title" :description="tfa.description" :token="tfa.token"
    :type="tfa.type" @confirm="onTfaConfirm" />
</template>

<style scoped>
.api-token-section {
  padding: 8px 20px 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.api-token-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.api-token-hint {
  margin: 0;
  font-size: 12.5px;
  opacity: 0.7;
  flex: 1;
  min-width: 200px;
}

.api-token-row {
  border: 1px solid rgba(128, 128, 128, 0.18);
  border-radius: 8px;
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  transition: opacity 0.15s;
}

.api-token-row.disabled {
  opacity: 0.55;
}

.api-token-row-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex-wrap: wrap;
}

.api-token-name-wrap {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.api-token-name {
  font-weight: 600;
  font-size: 13.5px;
}

.api-token-created {
  font-size: 11px;
  opacity: 0.55;
}

.api-token-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.api-token-value-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.api-token-value {
  flex: 1;
  min-width: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12.5px;
  padding: 4px 8px;
  background: rgba(128, 128, 128, 0.08);
  border-radius: 4px;
  word-break: break-all;
}
</style>
