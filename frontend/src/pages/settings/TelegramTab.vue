<script setup>
import { useI18n } from 'vue-i18n';
import { LanguageManager } from '@/utils';
import SettingListItem from '@/components/SettingListItem.vue';

const { t } = useI18n();

defineProps({
  allSetting: { type: Object, required: true },
});
</script>

<template>
  <a-collapse default-active-key="1">
    <a-collapse-panel key="1" :header="t('pages.settings.panelSettings')">
      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.telegramBotEnable') }}</template>
        <template #description>{{ t('pages.settings.telegramBotEnableDesc') }}</template>
        <template #control>
          <a-switch v-model:checked="allSetting.tgBotEnable" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.telegramToken') }}</template>
        <template #description>
          {{ allSetting.hasTgBotToken ? 'Configured; leave blank to keep current token.' : t('pages.settings.telegramTokenDesc') }}
        </template>
        <template #control>
          <a-input-password v-model:value="allSetting.tgBotToken"
            :placeholder="allSetting.hasTgBotToken ? 'Configured - enter a new token to replace' : ''" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.telegramChatId') }}</template>
        <template #description>{{ t('pages.settings.telegramChatIdDesc') }}</template>
        <template #control>
          <a-input v-model:value="allSetting.tgBotChatId" type="text" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.telegramBotLanguage') }}</template>
        <template #control>
          <a-select v-model:value="allSetting.tgLang" :style="{ width: '100%' }">
            <a-select-option v-for="l in LanguageManager.supportedLanguages" :key="l.value" :value="l.value"
              :label="l.value">
              <span role="img" :aria-label="l.name">{{ l.icon }}</span>
              &nbsp;&nbsp;<span>{{ l.name }}</span>
            </a-select-option>
          </a-select>
        </template>
      </SettingListItem>
    </a-collapse-panel>

    <a-collapse-panel key="2" :header="t('pages.settings.notifications')">
      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.telegramNotifyTime') }}</template>
        <template #description>{{ t('pages.settings.telegramNotifyTimeDesc') }}</template>
        <template #control>
          <a-input v-model:value="allSetting.tgRunTime" type="text" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.tgNotifyBackup') }}</template>
        <template #description>{{ t('pages.settings.tgNotifyBackupDesc') }}</template>
        <template #control>
          <a-switch v-model:checked="allSetting.tgBotBackup" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.tgNotifyLogin') }}</template>
        <template #description>{{ t('pages.settings.tgNotifyLoginDesc') }}</template>
        <template #control>
          <a-switch v-model:checked="allSetting.tgBotLoginNotify" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.tgNotifyCpu') }}</template>
        <template #description>{{ t('pages.settings.tgNotifyCpuDesc') }}</template>
        <template #control>
          <a-input-number v-model:value="allSetting.tgCpu" :min="0" :max="100" :style="{ width: '100%' }" />
        </template>
      </SettingListItem>
    </a-collapse-panel>

    <a-collapse-panel key="3" :header="t('pages.settings.proxyAndServer')">
      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.telegramProxy') }}</template>
        <template #description>{{ t('pages.settings.telegramProxyDesc') }}</template>
        <template #control>
          <a-input v-model:value="allSetting.tgBotProxy" type="text" placeholder="socks5://user:pass@host:port" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.telegramAPIServer') }}</template>
        <template #description>{{ t('pages.settings.telegramAPIServerDesc') }}</template>
        <template #control>
          <a-input v-model:value="allSetting.tgBotAPIServer" type="text" placeholder="https://api.example.com" />
        </template>
      </SettingListItem>
    </a-collapse-panel>
  </a-collapse>
</template>
