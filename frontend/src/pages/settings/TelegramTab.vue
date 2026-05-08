<script setup>
import { LanguageManager } from '@/utils';
import SettingListItem from '@/components/SettingListItem.vue';

defineProps({
  allSetting: { type: Object, required: true },
});
</script>

<template>
  <a-collapse default-active-key="1">
    <a-collapse-panel key="1" header="General">
      <SettingListItem paddings="small">
        <template #title>Enable Telegram bot</template>
        <template #description>Toggle the in-bot notification flow.</template>
        <template #control>
          <a-switch v-model:checked="allSetting.tgBotEnable" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Bot token</template>
        <template #description>Token issued by @BotFather.</template>
        <template #control>
          <a-input v-model:value="allSetting.tgBotToken" type="text" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Chat ID</template>
        <template #description>Telegram chat that receives notifications.</template>
        <template #control>
          <a-input v-model:value="allSetting.tgBotChatId" type="text" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Bot language</template>
        <template #control>
          <a-select v-model:value="allSetting.tgLang" :style="{ width: '100%' }">
            <a-select-option
              v-for="l in LanguageManager.supportedLanguages"
              :key="l.value"
              :value="l.value"
              :label="l.value"
            >
              <span role="img" :aria-label="l.name">{{ l.icon }}</span>
              &nbsp;&nbsp;<span>{{ l.name }}</span>
            </a-select-option>
          </a-select>
        </template>
      </SettingListItem>
    </a-collapse-panel>

    <a-collapse-panel key="2" header="Notifications">
      <SettingListItem paddings="small">
        <template #title>Notification schedule</template>
        <template #description>Cron expression — e.g. <code>@daily</code> or <code>0 0 * * *</code>.</template>
        <template #control>
          <a-input v-model:value="allSetting.tgRunTime" type="text" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Send database backup</template>
        <template #description>Attach a backup of x-ui.db on each scheduled notification.</template>
        <template #control>
          <a-switch v-model:checked="allSetting.tgBotBackup" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Notify on login</template>
        <template #description>Send a message whenever the panel is logged into.</template>
        <template #control>
          <a-switch v-model:checked="allSetting.tgBotLoginNotify" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>CPU notification threshold (%)</template>
        <template #description>Notify when CPU usage stays above this for a sustained window. 0 disables.</template>
        <template #control>
          <a-input-number
            v-model:value="allSetting.tgCpu"
            :min="0"
            :max="100"
            :style="{ width: '100%' }"
          />
        </template>
      </SettingListItem>
    </a-collapse-panel>

    <a-collapse-panel key="3" header="Proxy and API server">
      <SettingListItem paddings="small">
        <template #title>Bot proxy</template>
        <template #description>Outbound proxy used to reach the Telegram API.</template>
        <template #control>
          <a-input
            v-model:value="allSetting.tgBotProxy"
            type="text"
            placeholder="socks5://user:pass@host:port"
          />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Telegram API server</template>
        <template #description>Override the default api.telegram.org endpoint.</template>
        <template #control>
          <a-input
            v-model:value="allSetting.tgBotAPIServer"
            type="text"
            placeholder="https://api.example.com"
          />
        </template>
      </SettingListItem>
    </a-collapse-panel>
  </a-collapse>
</template>
