<script setup>
import { computed } from 'vue';
import { ExclamationCircleFilled, CloudOutlined, ApiOutlined } from '@ant-design/icons-vue';

import { OutboundDomainStrategies } from '@/models/outbound.js';
import SettingListItem from '@/components/SettingListItem.vue';

// Phase 6-ii: structured editor for the most-touched fields of the
// xray template — outbound strategy, routing strategy, log levels,
// stat counters, and the "basic routing" lists (block IPs/domains/
// torrent + direct IPs/domains + IPv4 forced + warp/nord domains).
//
// Mutates the parent's templateSettings reactive directly. The
// useXraySetting composable's deep watch on templateSettings re-
// stringifies into xraySetting so the Advanced JSON tab and the
// dirty-poll see every edit.

const props = defineProps({
  templateSettings: { type: Object, default: null },
  outboundTestUrl: { type: String, default: '' },
  warpExist: { type: Boolean, default: false },
  nordExist: { type: Boolean, default: false },
});

const emit = defineEmits(['update:outbound-test-url', 'show-warp', 'show-nord']);

// === Static option lists (mirror legacy) =============================
const ROUTING_DOMAIN_STRATEGIES = ['AsIs', 'IPIfNonMatch', 'IPOnDemand'];
const LOG_LEVELS = ['none', 'debug', 'info', 'warning', 'error'];
const ACCESS_LOG = ['none', './access.log'];
const ERROR_LOG = ['none', './error.log'];
const MASK_ADDRESS = ['quarter', 'half', 'full'];
const BITTORRENT_PROTOCOLS = ['bittorrent'];

// Country lists copied from the legacy template's settingsData. Keep
// them alphabetised by ISO code so additions stay obvious in diffs.
const IPS_OPTIONS = [
  { label: 'Private IPs', value: 'geoip:private' },
  { label: '🇮🇷 Iran', value: 'ext:geoip_IR.dat:ir' },
  { label: '🇨🇳 China', value: 'geoip:cn' },
  { label: '🇷🇺 Russia', value: 'ext:geoip_RU.dat:ru' },
];
const BLOCK_DOMAINS_OPTIONS = [
  { label: 'Ads (Iran)', value: 'ext:geosite_IR.dat:category-ads-all' },
  { label: 'Ads', value: 'geosite:category-ads-all' },
  { label: '🇮🇷 Iran', value: 'ext:geosite_IR.dat:category-ir' },
  { label: '🇨🇳 China', value: 'geosite:cn' },
  { label: '🇷🇺 Russia', value: 'ext:geosite_RU.dat:category-ru' },
];
const DOMAINS_OPTIONS = [
  { label: 'Private DNS', value: 'geosite:private' },
  { label: '🇮🇷 Iran', value: 'ext:geosite_IR.dat:category-ir' },
  { label: '🇨🇳 China', value: 'geosite:cn' },
  { label: '🇷🇺 Russia', value: 'ext:geosite_RU.dat:category-ru' },
];
const SERVICES_OPTIONS = [
  { label: 'Google', value: 'geosite:google' },
  { label: 'Apple', value: 'geosite:apple' },
  { label: 'Meta', value: 'geosite:meta' },
  { label: 'Microsoft', value: 'geosite:microsoft' },
  { label: 'Netflix', value: 'geosite:netflix' },
  { label: 'OpenAI', value: 'geosite:openai' },
  { label: 'Spotify', value: 'geosite:spotify' },
];

// === Routing-rule helpers (matches legacy templateRule{Getter,Setter}) ==
function ruleGetter(outboundTag, property) {
  if (!props.templateSettings?.routing?.rules) return [];
  const out = [];
  for (const rule of props.templateSettings.routing.rules) {
    if (
      rule
      && Object.prototype.hasOwnProperty.call(rule, property)
      && Object.prototype.hasOwnProperty.call(rule, 'outboundTag')
      && rule.outboundTag === outboundTag
    ) {
      out.push(...rule[property]);
    }
  }
  return out;
}
function ruleSetter(outboundTag, property, data) {
  if (!props.templateSettings?.routing) return;
  const current = ruleGetter(outboundTag, property);
  if (current.length === 0) {
    props.templateSettings.routing.rules.push({
      type: 'field',
      outboundTag,
      [property]: data,
    });
    return;
  }
  // Replace the property on the FIRST matching rule and drop any later
  // duplicates with the same (outboundTag, property) pair (matches the
  // legacy single-write-then-filter behavior).
  const next = [];
  let inserted = false;
  for (const rule of props.templateSettings.routing.rules) {
    const matches =
      rule
      && Object.prototype.hasOwnProperty.call(rule, property)
      && Object.prototype.hasOwnProperty.call(rule, 'outboundTag')
      && rule.outboundTag === outboundTag;
    if (matches) {
      if (!inserted && data.length > 0) {
        rule[property] = data;
        next.push(rule);
        inserted = true;
      }
    } else {
      next.push(rule);
    }
  }
  props.templateSettings.routing.rules = next;
}

function syncOutbound(tag, settings) {
  // After editing direct/IPv4/warp/nord rules, ensure the matching
  // outbound exists when the rule list has any entries, and is
  // pruned when none remain (legacy syncRulesWithOutbound).
  const t = props.templateSettings;
  if (!t) return;
  const haveRules = t.routing.rules.some((r) => r?.outboundTag === tag);
  const idx = t.outbounds.findIndex((o) => o.tag === tag);
  if (!haveRules && idx > 0) t.outbounds.splice(idx, 1);
  if (haveRules && idx < 0) t.outbounds.push(settings);
}

// === Computed v-models for every Basics field ========================
function rule(tag, property, syncFn) {
  return computed({
    get: () => ruleGetter(tag, property),
    set: (next) => { ruleSetter(tag, property, next); if (syncFn) syncFn(); },
  });
}

const directSettings = { tag: 'direct', protocol: 'freedom' };
const ipv4Settings = { tag: 'IPv4', protocol: 'freedom', settings: { domainStrategy: 'UseIPv4' } };

const freedomStrategy = computed({
  get: () => {
    const ob = props.templateSettings?.outbounds?.find(
      (o) => o.protocol === 'freedom' && o.tag === 'direct',
    );
    return ob?.settings?.domainStrategy ?? 'AsIs';
  },
  set: (next) => {
    const t = props.templateSettings;
    if (!t) return;
    const idx = t.outbounds.findIndex((o) => o.protocol === 'freedom' && o.tag === 'direct');
    if (idx < 0) {
      t.outbounds.push({ protocol: 'freedom', tag: 'direct', settings: { domainStrategy: next } });
    } else {
      t.outbounds[idx].settings = t.outbounds[idx].settings || {};
      t.outbounds[idx].settings.domainStrategy = next;
    }
  },
});

const routingStrategy = computed({
  get: () => props.templateSettings?.routing?.domainStrategy ?? 'AsIs',
  set: (next) => { if (props.templateSettings?.routing) props.templateSettings.routing.domainStrategy = next; },
});

function logField(field, fallback) {
  return computed({
    get: () => props.templateSettings?.log?.[field] ?? fallback,
    set: (next) => { if (props.templateSettings?.log) props.templateSettings.log[field] = next; },
  });
}
const logLevel = logField('loglevel', 'warning');
const accessLog = logField('access', '');
const errorLog = logField('error', '');
const maskAddressLog = logField('maskAddress', '');
const dnslog = logField('dnsLog', false);

function policyField(field) {
  return computed({
    get: () => !!props.templateSettings?.policy?.system?.[field],
    set: (next) => {
      if (!props.templateSettings?.policy?.system) return;
      props.templateSettings.policy.system[field] = next;
    },
  });
}
const statsInboundUplink = policyField('statsInboundUplink');
const statsInboundDownlink = policyField('statsInboundDownlink');
const statsOutboundUplink = policyField('statsOutboundUplink');
const statsOutboundDownlink = policyField('statsOutboundDownlink');

const blockedIPs = rule('blocked', 'ip');
const blockedDomains = rule('blocked', 'domain');
const blockedProtocols = rule('blocked', 'protocol');
const directIPs = rule('direct', 'ip', () => syncOutbound('direct', directSettings));
const directDomains = rule('direct', 'domain', () => syncOutbound('direct', directSettings));
const ipv4Domains = rule('IPv4', 'domain', () => syncOutbound('IPv4', ipv4Settings));
const warpDomains = rule('warp', 'domain');
const nordTag = computed(() => {
  const ob = props.templateSettings?.outbounds?.find((o) => o.tag?.startsWith?.('nord-'));
  return ob?.tag || 'nord';
});
const nordDomains = computed({
  get: () => ruleGetter(nordTag.value, 'domain'),
  set: (next) => ruleSetter(nordTag.value, 'domain', next),
});

const torrentSettings = computed({
  get: () => BITTORRENT_PROTOCOLS.every((p) => blockedProtocols.value.includes(p)),
  set: (next) => {
    if (next) {
      blockedProtocols.value = [...blockedProtocols.value, ...BITTORRENT_PROTOCOLS];
    } else {
      blockedProtocols.value = blockedProtocols.value.filter((d) => !BITTORRENT_PROTOCOLS.includes(d));
    }
  },
});

const localOutboundTestUrl = computed({
  get: () => props.outboundTestUrl,
  set: (next) => emit('update:outbound-test-url', next),
});
</script>

<template>
  <a-collapse default-active-key="1">
    <a-collapse-panel key="1" header="General">
      <a-alert
        type="warning"
        class="mb-12 hint-alert"
        message="Editing this template manually is for advanced users only."
      >
        <template #icon><ExclamationCircleFilled style="color: #FFA031;" /></template>
      </a-alert>

      <SettingListItem paddings="small">
        <template #title>Freedom outbound strategy</template>
        <template #description>How the direct outbound resolves destinations.</template>
        <template #control>
          <a-select v-model:value="freedomStrategy" :style="{ width: '100%' }">
            <a-select-option v-for="s in OutboundDomainStrategies" :key="s" :value="s">{{ s }}</a-select-option>
          </a-select>
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Routing strategy</template>
        <template #description>Domain strategy applied at the routing level.</template>
        <template #control>
          <a-select v-model:value="routingStrategy" :style="{ width: '100%' }">
            <a-select-option v-for="s in ROUTING_DOMAIN_STRATEGIES" :key="s" :value="s">{{ s }}</a-select-option>
          </a-select>
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Outbound test URL</template>
        <template #description>HTTP endpoint used for outbound latency tests.</template>
        <template #control>
          <a-input v-model:value="localOutboundTestUrl" placeholder="https://www.google.com/generate_204" />
        </template>
      </SettingListItem>
    </a-collapse-panel>

    <a-collapse-panel key="2" header="Statistics">
      <SettingListItem paddings="small">
        <template #title>Inbound uplink stats</template>
        <template #control><a-switch v-model:checked="statsInboundUplink" /></template>
      </SettingListItem>
      <SettingListItem paddings="small">
        <template #title>Inbound downlink stats</template>
        <template #control><a-switch v-model:checked="statsInboundDownlink" /></template>
      </SettingListItem>
      <SettingListItem paddings="small">
        <template #title>Outbound uplink stats</template>
        <template #control><a-switch v-model:checked="statsOutboundUplink" /></template>
      </SettingListItem>
      <SettingListItem paddings="small">
        <template #title>Outbound downlink stats</template>
        <template #control><a-switch v-model:checked="statsOutboundDownlink" /></template>
      </SettingListItem>
    </a-collapse-panel>

    <a-collapse-panel key="3" header="Logs">
      <a-alert
        type="warning"
        class="mb-12 hint-alert"
        message="Empty access/error log fields disable that log; ./access.log writes to disk."
      >
        <template #icon><ExclamationCircleFilled style="color: #FFA031;" /></template>
      </a-alert>

      <SettingListItem paddings="small">
        <template #title>Log level</template>
        <template #control>
          <a-select v-model:value="logLevel" :style="{ width: '100%' }">
            <a-select-option v-for="s in LOG_LEVELS" :key="s" :value="s">{{ s }}</a-select-option>
          </a-select>
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Access log</template>
        <template #control>
          <a-select v-model:value="accessLog" :style="{ width: '100%' }">
            <a-select-option value="">Empty</a-select-option>
            <a-select-option v-for="s in ACCESS_LOG" :key="s" :value="s">{{ s }}</a-select-option>
          </a-select>
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Error log</template>
        <template #control>
          <a-select v-model:value="errorLog" :style="{ width: '100%' }">
            <a-select-option value="">Empty</a-select-option>
            <a-select-option v-for="s in ERROR_LOG" :key="s" :value="s">{{ s }}</a-select-option>
          </a-select>
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Mask address</template>
        <template #description>Truncate IPs in logs.</template>
        <template #control>
          <a-select v-model:value="maskAddressLog" :style="{ width: '100%' }">
            <a-select-option value="">Empty</a-select-option>
            <a-select-option v-for="s in MASK_ADDRESS" :key="s" :value="s">{{ s }}</a-select-option>
          </a-select>
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>DNS log</template>
        <template #control><a-switch v-model:checked="dnslog" /></template>
      </SettingListItem>
    </a-collapse-panel>

    <a-collapse-panel key="4" header="Basic routing">
      <a-alert
        type="warning"
        class="mb-12 hint-alert"
        message="These shortcuts edit the underlying routing rules. Use the Routing tab for full control."
      >
        <template #icon><ExclamationCircleFilled style="color: #FFA031;" /></template>
      </a-alert>

      <SettingListItem paddings="small">
        <template #title>Block torrent</template>
        <template #control><a-switch v-model:checked="torrentSettings" /></template>
      </SettingListItem>

      <a-alert
        type="warning"
        class="mb-12 hint-alert"
        message="Block lists drop traffic matching these IPs / domains."
      >
        <template #icon><ExclamationCircleFilled style="color: #FFA031;" /></template>
      </a-alert>

      <SettingListItem paddings="small">
        <template #title>Block IPs</template>
        <template #control>
          <a-select v-model:value="blockedIPs" mode="tags" :style="{ width: '100%' }">
            <a-select-option v-for="p in IPS_OPTIONS" :key="p.value" :value="p.value" :label="p.label">{{ p.label }}</a-select-option>
          </a-select>
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Block domains</template>
        <template #control>
          <a-select v-model:value="blockedDomains" mode="tags" :style="{ width: '100%' }">
            <a-select-option v-for="p in BLOCK_DOMAINS_OPTIONS" :key="p.value" :value="p.value" :label="p.label">{{ p.label }}</a-select-option>
          </a-select>
        </template>
      </SettingListItem>

      <a-alert
        type="warning"
        class="mb-12 hint-alert"
        message="Direct lists bypass the proxy for matched IPs / domains."
      >
        <template #icon><ExclamationCircleFilled style="color: #FFA031;" /></template>
      </a-alert>

      <SettingListItem paddings="small">
        <template #title>Direct IPs</template>
        <template #control>
          <a-select v-model:value="directIPs" mode="tags" :style="{ width: '100%' }">
            <a-select-option v-for="p in IPS_OPTIONS" :key="p.value" :value="p.value" :label="p.label">{{ p.label }}</a-select-option>
          </a-select>
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>Direct domains</template>
        <template #control>
          <a-select v-model:value="directDomains" mode="tags" :style="{ width: '100%' }">
            <a-select-option v-for="p in DOMAINS_OPTIONS" :key="p.value" :value="p.value" :label="p.label">{{ p.label }}</a-select-option>
          </a-select>
        </template>
      </SettingListItem>

      <a-alert
        type="warning"
        class="mb-12 hint-alert"
        message="IPv4 routing forces the listed services through an IPv4-only outbound."
      >
        <template #icon><ExclamationCircleFilled style="color: #FFA031;" /></template>
      </a-alert>

      <SettingListItem paddings="small">
        <template #title>IPv4 forced</template>
        <template #control>
          <a-select v-model:value="ipv4Domains" mode="tags" :style="{ width: '100%' }">
            <a-select-option v-for="p in SERVICES_OPTIONS" :key="p.value" :value="p.value" :label="p.label">{{ p.label }}</a-select-option>
          </a-select>
        </template>
      </SettingListItem>

      <a-alert
        type="warning"
        class="mb-12 hint-alert"
        message="WARP / NordVPN routing requires the matching outbound to exist — use the buttons to provision it."
      >
        <template #icon><ExclamationCircleFilled style="color: #FFA031;" /></template>
      </a-alert>

      <SettingListItem paddings="small">
        <template #title>WARP routing</template>
        <template #control>
          <a-select
            v-if="warpExist"
            v-model:value="warpDomains"
            mode="tags"
            :style="{ width: '100%' }"
          >
            <a-select-option v-for="p in SERVICES_OPTIONS" :key="p.value" :value="p.value" :label="p.label">{{ p.label }}</a-select-option>
          </a-select>
          <a-button v-else type="primary" @click="emit('show-warp')">
            <template #icon><CloudOutlined /></template>
            Configure WARP
          </a-button>
        </template>
      </SettingListItem>

      <SettingListItem paddings="small">
        <template #title>NordVPN routing</template>
        <template #control>
          <a-select
            v-if="nordExist"
            v-model:value="nordDomains"
            mode="tags"
            :style="{ width: '100%' }"
          >
            <a-select-option v-for="p in SERVICES_OPTIONS" :key="p.value" :value="p.value" :label="p.label">{{ p.label }}</a-select-option>
          </a-select>
          <a-button v-else type="primary" @click="emit('show-nord')">
            <template #icon><ApiOutlined /></template>
            Configure NordVPN
          </a-button>
        </template>
      </SettingListItem>
    </a-collapse-panel>
  </a-collapse>
</template>

<style scoped>
.mb-12 { margin-bottom: 12px; }
.hint-alert { text-align: center; }
</style>
