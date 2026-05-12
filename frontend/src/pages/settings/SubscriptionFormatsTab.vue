<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import SettingListItem from '@/components/SettingListItem.vue';

const { t } = useI18n();

const props = defineProps({
  allSetting: { type: Object, required: true },
});

// === Defaults (match legacy) ============================================
const DEFAULT_FRAGMENT = {
  packets: 'tlshello',
  length: '100-200',
  interval: '10-20',
  maxSplit: '300-400',
};
const DEFAULT_NOISES = [{ type: 'rand', packet: '10-20', delay: '10-16', applyTo: 'ip' }];
const DEFAULT_MUX = {
  enabled: true,
  concurrency: 8,
  xudpConcurrency: 16,
  xudpProxyUDP443: 'reject',
};
const DEFAULT_RULES = [
  { type: 'field', outboundTag: 'direct', domain: ['geosite:category-ir'] },
  { type: 'field', outboundTag: 'direct', ip: ['geoip:private', 'geoip:ir'] },
];

const directIPsOptions = [
  { label: 'Private IP', value: 'geoip:private' },
  { label: '🇮🇷 Iran', value: 'geoip:ir' },
  { label: '🇨🇳 China', value: 'geoip:cn' },
  { label: '🇷🇺 Russia', value: 'geoip:ru' },
  { label: '🇻🇳 Vietnam', value: 'geoip:vn' },
  { label: '🇪🇸 Spain', value: 'geoip:es' },
  { label: '🇮🇩 Indonesia', value: 'geoip:id' },
  { label: '🇺🇦 Ukraine', value: 'geoip:ua' },
  { label: '🇹🇷 Türkiye', value: 'geoip:tr' },
  { label: '🇧🇷 Brazil', value: 'geoip:br' },
];
const directDomainsOptions = [
  { label: 'Private DNS', value: 'geosite:private' },
  { label: '🇮🇷 Iran', value: 'geosite:category-ir' },
  { label: '🇨🇳 China', value: 'geosite:cn' },
  { label: '🇷🇺 Russia', value: 'geosite:category-ru' },
  { label: 'Apple', value: 'geosite:apple' },
  { label: 'Meta', value: 'geosite:meta' },
  { label: 'Google', value: 'geosite:google' },
];

// === Path helpers (json + clash share the same shape) ===================
function makePath(field) {
  return computed({
    get: () => props.allSetting[field],
    set: (v) => {
      props.allSetting[field] = String(v ?? '').replace(/[:*]/g, '');
    },
  });
}
function normalizePath(field) {
  let p = props.allSetting[field] || '/';
  if (!p.startsWith('/')) p = '/' + p;
  if (!p.endsWith('/')) p += '/';
  p = p.replace(/\/+/g, '/');
  props.allSetting[field] = p;
}
const subJsonPath = makePath('subJsonPath');
const subClashPath = makePath('subClashPath');

// === Fragment ===========================================================
// `subJsonFragment` is a JSON-encoded object when enabled, "" when off.
function readJson(field, fallback) {
  try {
    const raw = props.allSetting[field];
    if (!raw) return fallback;
    return JSON.parse(raw);
  } catch (_e) {
    return fallback;
  }
}
function writeJson(field, value) {
  props.allSetting[field] = JSON.stringify(value);
}

const fragment = computed({
  get: () => props.allSetting.subJsonFragment !== '',
  set: (v) => {
    props.allSetting.subJsonFragment = v ? JSON.stringify(DEFAULT_FRAGMENT) : '';
  },
});
function makeFragmentField(key) {
  return computed({
    get: () => (fragment.value ? readJson('subJsonFragment', DEFAULT_FRAGMENT)[key] : ''),
    set: (v) => {
      if (v === '') return;
      const f = readJson('subJsonFragment', { ...DEFAULT_FRAGMENT });
      f[key] = v;
      writeJson('subJsonFragment', f);
    },
  });
}
const fragmentPackets = makeFragmentField('packets');
const fragmentLength = makeFragmentField('length');
const fragmentInterval = makeFragmentField('interval');
const fragmentMaxSplit = makeFragmentField('maxSplit');

// === Noises =============================================================
const noises = computed({
  get: () => props.allSetting.subJsonNoises !== '',
  set: (v) => {
    props.allSetting.subJsonNoises = v ? JSON.stringify(DEFAULT_NOISES) : '';
  },
});
const noisesArray = computed({
  get: () => (noises.value ? readJson('subJsonNoises', DEFAULT_NOISES) : []),
  set: (value) => { if (noises.value) writeJson('subJsonNoises', value); },
});
function addNoise() {
  noisesArray.value = [...noisesArray.value, { ...DEFAULT_NOISES[0] }];
}
function removeNoise(index) {
  const next = [...noisesArray.value];
  next.splice(index, 1);
  noisesArray.value = next;
}
function updateNoiseField(index, field, value) {
  const next = [...noisesArray.value];
  next[index] = { ...next[index], [field]: value };
  noisesArray.value = next;
}

// === Mux ================================================================
const enableMux = computed({
  get: () => props.allSetting.subJsonMux !== '',
  set: (v) => {
    props.allSetting.subJsonMux = v ? JSON.stringify(DEFAULT_MUX) : '';
  },
});
function makeMuxField(key, fallback) {
  return computed({
    get: () => (enableMux.value ? readJson('subJsonMux', DEFAULT_MUX)[key] : fallback),
    set: (v) => {
      const m = readJson('subJsonMux', { ...DEFAULT_MUX });
      m[key] = v;
      writeJson('subJsonMux', m);
    },
  });
}
const muxConcurrency = makeMuxField('concurrency', -1);
const muxXudpConcurrency = makeMuxField('xudpConcurrency', -1);
const muxXudpProxyUDP443 = makeMuxField('xudpProxyUDP443', 'reject');

// === Direct routing rules ==============================================
// `subJsonRules` is a JSON array of xray routing rules. We surface the
// IP and domain fields of the two seed rules as multi-select tags.
const enableDirect = computed({
  get: () => props.allSetting.subJsonRules !== '',
  set: (v) => {
    props.allSetting.subJsonRules = v ? JSON.stringify(DEFAULT_RULES) : '';
  },
});
function ruleArray() {
  if (!enableDirect.value) return null;
  const rules = readJson('subJsonRules', null);
  return Array.isArray(rules) ? rules : null;
}
const directIPs = computed({
  get: () => {
    const rules = ruleArray();
    if (!rules) return [];
    const ipRule = rules.find((r) => r.ip);
    return ipRule?.ip ?? [];
  },
  set: (value) => {
    let rules = ruleArray();
    if (!rules) return;
    if (value.length === 0) {
      rules = rules.filter((r) => !r.ip);
    } else {
      let idx = rules.findIndex((r) => r.ip);
      if (idx === -1) idx = rules.push({ ...DEFAULT_RULES[1] }) - 1;
      rules[idx].ip = [...value];
    }
    writeJson('subJsonRules', rules);
  },
});
const directDomains = computed({
  get: () => {
    const rules = ruleArray();
    if (!rules) return [];
    const dRule = rules.find((r) => r.domain);
    return dRule?.domain ?? [];
  },
  set: (value) => {
    let rules = ruleArray();
    if (!rules) return;
    if (value.length === 0) {
      rules = rules.filter((r) => !r.domain);
    } else {
      let idx = rules.findIndex((r) => r.domain);
      if (idx === -1) idx = rules.push({ ...DEFAULT_RULES[0] }) - 1;
      rules[idx].domain = [...value];
    }
    writeJson('subJsonRules', rules);
  },
});
</script>

<template>
  <a-collapse default-active-key="1">
    <a-collapse-panel key="1" :header="t('pages.settings.panelSettings')">
      <SettingListItem v-if="allSetting.subJsonEnable" paddings="small">
        <template #title>JSON {{ t('pages.settings.subPath') }}</template>
        <template #description>{{ t('pages.settings.subPathDesc') }}</template>
        <template #control>
          <a-input v-model:value="subJsonPath" type="text" placeholder="/json/" @blur="normalizePath('subJsonPath')" />
        </template>
      </SettingListItem>

      <SettingListItem v-if="allSetting.subJsonEnable" paddings="small">
        <template #title>JSON {{ t('pages.settings.subURI') }}</template>
        <template #description>{{ t('pages.settings.subURIDesc') }}</template>
        <template #control>
          <a-input v-model:value="allSetting.subJsonURI" type="text" placeholder="(http|https)://domain[:port]/path/" />
        </template>
      </SettingListItem>

      <SettingListItem v-if="allSetting.subClashEnable" paddings="small">
        <template #title>Clash {{ t('pages.settings.subPath') }}</template>
        <template #description>{{ t('pages.settings.subPathDesc') }}</template>
        <template #control>
          <a-input v-model:value="subClashPath" type="text" placeholder="/clash/"
            @blur="normalizePath('subClashPath')" />
        </template>
      </SettingListItem>

      <SettingListItem v-if="allSetting.subClashEnable" paddings="small">
        <template #title>Clash {{ t('pages.settings.subURI') }}</template>
        <template #description>{{ t('pages.settings.subURIDesc') }}</template>
        <template #control>
          <a-input v-model:value="allSetting.subClashURI" type="text"
            placeholder="(http|https)://domain[:port]/path/" />
        </template>
      </SettingListItem>
    </a-collapse-panel>

    <a-collapse-panel key="2" :header="t('pages.settings.fragment')">
      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.fragment') }}</template>
        <template #description>{{ t('pages.settings.fragmentDesc') }}</template>
        <template #control>
          <a-switch v-model:checked="fragment" />
        </template>
      </SettingListItem>

      <a-list-item v-if="fragment" class="nested-block">
        <a-collapse>
          <a-collapse-panel :header="t('pages.settings.fragmentSett')">
            <SettingListItem paddings="small">
              <template #title>Packets</template>
              <template #control>
                <a-input v-model:value="fragmentPackets" placeholder="1-1 | 1-3 | tlshello | …" />
              </template>
            </SettingListItem>
            <SettingListItem paddings="small">
              <template #title>Length</template>
              <template #control>
                <a-input v-model:value="fragmentLength" placeholder="100-200" />
              </template>
            </SettingListItem>
            <SettingListItem paddings="small">
              <template #title>Interval</template>
              <template #control>
                <a-input v-model:value="fragmentInterval" placeholder="10-20" />
              </template>
            </SettingListItem>
            <SettingListItem paddings="small">
              <template #title>Max split</template>
              <template #control>
                <a-input v-model:value="fragmentMaxSplit" placeholder="300-400" />
              </template>
            </SettingListItem>
          </a-collapse-panel>
        </a-collapse>
      </a-list-item>
    </a-collapse-panel>

    <a-collapse-panel key="3" header="Noises">
      <SettingListItem paddings="small">
        <template #title>Noises</template>
        <template #description>{{ t('pages.settings.noisesDesc') }}</template>
        <template #control>
          <a-switch v-model:checked="noises" />
        </template>
      </SettingListItem>

      <a-list-item v-if="noises" class="nested-block">
        <a-collapse>
          <a-collapse-panel v-for="(noise, index) in noisesArray" :key="index" :header="`Noise №${index + 1}`">
            <SettingListItem paddings="small">
              <template #title>Type</template>
              <template #control>
                <a-select :value="noise.type" :style="{ width: '100%' }"
                  @change="(v) => updateNoiseField(index, 'type', v)">
                  <a-select-option v-for="p in ['rand', 'base64', 'str', 'hex']" :key="p" :value="p">
                    {{ p }}
                  </a-select-option>
                </a-select>
              </template>
            </SettingListItem>
            <SettingListItem paddings="small">
              <template #title>Packet</template>
              <template #control>
                <a-input :value="noise.packet" placeholder="5-10"
                  @input="(e) => updateNoiseField(index, 'packet', e.target.value)" />
              </template>
            </SettingListItem>
            <SettingListItem paddings="small">
              <template #title>Delay (ms)</template>
              <template #control>
                <a-input :value="noise.delay" placeholder="10-20"
                  @input="(e) => updateNoiseField(index, 'delay', e.target.value)" />
              </template>
            </SettingListItem>
            <SettingListItem paddings="small">
              <template #title>Apply to</template>
              <template #control>
                <a-select :value="noise.applyTo" :style="{ width: '100%' }"
                  @change="(v) => updateNoiseField(index, 'applyTo', v)">
                  <a-select-option v-for="p in ['ip', 'ipv4', 'ipv6']" :key="p" :value="p">
                    {{ p }}
                  </a-select-option>
                </a-select>
              </template>
            </SettingListItem>

            <a-space direction="horizontal" :style="{ padding: '10px 20px' }">
              <a-button v-if="noisesArray.length > 1" type="primary" danger @click="removeNoise(index)">
                {{ t('delete') }}
              </a-button>
            </a-space>
          </a-collapse-panel>
        </a-collapse>

        <a-button type="primary" :style="{ marginTop: '10px' }" @click="addNoise">+ Noise</a-button>
      </a-list-item>
    </a-collapse-panel>

    <a-collapse-panel key="4" :header="t('pages.settings.mux')">
      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.mux') }}</template>
        <template #description>{{ t('pages.settings.muxDesc') }}</template>
        <template #control>
          <a-switch v-model:checked="enableMux" />
        </template>
      </SettingListItem>

      <a-list-item v-if="enableMux" class="nested-block">
        <a-collapse>
          <a-collapse-panel :header="t('pages.settings.muxSett')">
            <SettingListItem paddings="small">
              <template #title>Concurrency</template>
              <template #control>
                <a-input-number v-model:value="muxConcurrency" :min="-1" :max="1024" :style="{ width: '100%' }" />
              </template>
            </SettingListItem>
            <SettingListItem paddings="small">
              <template #title>xudp concurrency</template>
              <template #control>
                <a-input-number v-model:value="muxXudpConcurrency" :min="-1" :max="1024" :style="{ width: '100%' }" />
              </template>
            </SettingListItem>
            <SettingListItem paddings="small">
              <template #title>xudp UDP 443</template>
              <template #control>
                <a-select v-model:value="muxXudpProxyUDP443" :style="{ width: '100%' }">
                  <a-select-option v-for="p in ['reject', 'allow', 'skip']" :key="p" :value="p">
                    {{ p }}
                  </a-select-option>
                </a-select>
              </template>
            </SettingListItem>
          </a-collapse-panel>
        </a-collapse>
      </a-list-item>
    </a-collapse-panel>

    <a-collapse-panel key="5" :header="t('pages.settings.direct')">
      <SettingListItem paddings="small">
        <template #title>{{ t('pages.settings.direct') }}</template>
        <template #description>{{ t('pages.settings.directDesc') }}</template>
        <template #control>
          <a-switch v-model:checked="enableDirect" />
        </template>
      </SettingListItem>

      <a-list-item v-if="enableDirect" class="nested-block">
        <a-collapse>
          <a-collapse-panel :header="t('pages.settings.direct')">
            <SettingListItem paddings="small">
              <template #title>{{ t('pages.settings.direct') }} IPs</template>
              <template #control>
                <a-select v-model:value="directIPs" mode="tags" :style="{ width: '100%' }">
                  <a-select-option v-for="p in directIPsOptions" :key="p.value" :value="p.value" :label="p.label">
                    {{ p.label }}
                  </a-select-option>
                </a-select>
              </template>
            </SettingListItem>
            <SettingListItem paddings="small">
              <template #title>{{ t('pages.settings.direct') }} {{ t('domainName') }}</template>
              <template #control>
                <a-select v-model:value="directDomains" mode="tags" :style="{ width: '100%' }">
                  <a-select-option v-for="p in directDomainsOptions" :key="p.value" :value="p.value" :label="p.label">
                    {{ p.label }}
                  </a-select-option>
                </a-select>
              </template>
            </SettingListItem>
          </a-collapse-panel>
        </a-collapse>
      </a-list-item>
    </a-collapse-panel>
  </a-collapse>
</template>

<style scoped>
.nested-block {
  padding: 10px 20px;
}
</style>
