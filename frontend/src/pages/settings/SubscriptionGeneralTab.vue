<script setup>
import { computed } from 'vue';
import SettingListItem from '@/components/SettingListItem.vue';

const props = defineProps({
  allSetting: { type: Object, required: true },
});

// Sub path is constrained: no `:` or `*`, must start and end with `/`,
// and no double slashes. Strip on input, normalize on blur — same
// behavior as the legacy template.
const subPath = computed({
  get: () => props.allSetting.subPath,
  set: (v) => {
    props.allSetting.subPath = String(v ?? '').replace(/[:*]/g, '');
  },
});

function normalizeSubPath() {
  let p = props.allSetting.subPath || '/';
  if (!p.startsWith('/')) p = '/' + p;
  if (!p.endsWith('/')) p += '/';
  p = p.replace(/\/+/g, '/');
  props.allSetting.subPath = p;
}
</script>

<template>
  <a-collapse default-active-key="1">
    <a-collapse-panel key="1" header="General">
      <SettingListItem paddings="small">
        <template #title>Subscription enable</template>
        <template #description>Master switch for /sub endpoints.</template>
        <template #control>
          <a-switch v-model:checked="allSetting.subEnable" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>JSON subscription</template>
        <template #description>Expose /json subscription endpoints alongside /sub.</template>
        <template #control>
          <a-switch v-model:checked="allSetting.subJsonEnable" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Clash / Mihomo subscription</template>
        <template #description>Enable direct Clash and Mihomo YAML subscriptions.</template>
        <template #control>
          <a-switch v-model:checked="allSetting.subClashEnable" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Subscription listen IP</template>
        <template #description>The IP the subscription server binds to. Leave empty to share the panel listener.</template>
        <template #control>
          <a-input v-model:value="allSetting.subListen" type="text" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Subscription domain</template>
        <template #description>Domain returned in subscription URLs.</template>
        <template #control>
          <a-input v-model:value="allSetting.subDomain" type="text" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Subscription port</template>
        <template #description>Restart required after changing.</template>
        <template #control>
          <a-input-number
            v-model:value="allSetting.subPort"
            :min="1"
            :max="65535"
            :style="{ width: '100%' }"
          />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Subscription path</template>
        <template #description>URL prefix for subscription endpoints (must start and end with /).</template>
        <template #control>
          <a-input
            v-model:value="subPath"
            type="text"
            placeholder="/sub/"
            @blur="normalizeSubPath"
          />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Subscription URI override</template>
        <template #description>Full URL returned to clients — overrides scheme/domain/port/path when set.</template>
        <template #control>
          <a-input
            v-model:value="allSetting.subURI"
            type="text"
            placeholder="(http|https)://domain[:port]/path/"
          />
        </template>
      </SettingListItem>
    </a-collapse-panel>

    <a-collapse-panel key="2" header="Information">
      <SettingListItem paddings="small">
        <template #title>Encrypt subscription</template>
        <template #description>Encrypt subscription content; clients need the matching key.</template>
        <template #control>
          <a-switch v-model:checked="allSetting.subEncrypt" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Show usage info</template>
        <template #description>Include used/total traffic and expiry in the subscription headers.</template>
        <template #control>
          <a-switch v-model:checked="allSetting.subShowInfo" />
        </template>
      </SettingListItem>

      <a-divider>Basic template</a-divider>

      <SettingListItem paddings="small">
        <template #title>Title</template>
        <template #description>Subscription title shown in clients.</template>
        <template #control>
          <a-input v-model:value="allSetting.subTitle" type="text" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Support URL</template>
        <template #description>Link surfaced to clients for support.</template>
        <template #control>
          <a-input v-model:value="allSetting.subSupportUrl" type="text" placeholder="https://example.com" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Profile URL</template>
        <template #description>Profile/announcement URL surfaced to clients.</template>
        <template #control>
          <a-input v-model:value="allSetting.subProfileUrl" type="text" placeholder="https://example.com" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Announce</template>
        <template #description>Free-form announcement appended to the subscription header.</template>
        <template #control>
          <a-textarea v-model:value="allSetting.subAnnounce" />
        </template>
      </SettingListItem>

      <a-divider>Advanced template (Happ)</a-divider>

      <SettingListItem paddings="small">
        <template #title>Enable Happ routing</template>
        <template #description>Embed Happ routing rules in the subscription.</template>
        <template #control>
          <a-switch v-model:checked="allSetting.subEnableRouting" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Routing rules</template>
        <template #description>One happ:// directive per line.</template>
        <template #control>
          <a-textarea
            v-model:value="allSetting.subRoutingRules"
            placeholder="happ://routing/add/..."
          />
        </template>
      </SettingListItem>
    </a-collapse-panel>

    <a-collapse-panel key="3" header="Certificates">
      <SettingListItem paddings="small">
        <template #title>Subscription cert path</template>
        <template #description>Absolute path to the subscription server's TLS certificate.</template>
        <template #control>
          <a-input v-model:value="allSetting.subCertFile" type="text" />
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Subscription key path</template>
        <template #description>Absolute path to the subscription server's private key.</template>
        <template #control>
          <a-input v-model:value="allSetting.subKeyFile" type="text" />
        </template>
      </SettingListItem>
    </a-collapse-panel>

    <a-collapse-panel key="4" header="Update interval">
      <SettingListItem paddings="small">
        <template #title>Update hours</template>
        <template #description>Hours clients should wait before re-fetching the subscription.</template>
        <template #control>
          <a-input-number
            v-model:value="allSetting.subUpdates"
            :min="1"
            :style="{ width: '100%' }"
          />
        </template>
      </SettingListItem>
    </a-collapse-panel>
  </a-collapse>
</template>
