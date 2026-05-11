<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  BarsOutlined,
  PoweroffOutlined,
  ReloadOutlined,
  ToolOutlined,
} from '@ant-design/icons-vue';

const { t } = useI18n();

const props = defineProps({
  status: { type: Object, required: true },
  isMobile: { type: Boolean, default: false },
  ipLimitEnable: { type: Boolean, default: false },
});

defineEmits(['stop-xray', 'restart-xray', 'open-logs', 'open-xray-logs', 'open-version-switch']);

const XRAY_STATE_KEYS = {
  running: 'pages.index.xrayStatusRunning',
  stop: 'pages.index.xrayStatusStop',
  error: 'pages.index.xrayStatusError',
};

const stateText = computed(() =>
  t(XRAY_STATE_KEYS[props.status.xray.state] ?? 'pages.index.xrayStatusUnknown'),
);

function badgeAnimationClass(color) {
  if (color === 'green') return 'xray-running-animation';
  if (color === 'orange') return 'xray-stop-animation';
  if (color === 'red') return 'xray-error-animation';
  return 'xray-processing-animation';
}
</script>

<template>
  <a-card hoverable>
    <template #title>
      <a-space direction="horizontal">
        <span>{{ t('pages.index.xrayStatus') }}</span>
        <a-tag v-if="isMobile && status.xray.version && status.xray.version !== 'Unknown'" color="green">
          v{{ status.xray.version }}
        </a-tag>
      </a-space>
    </template>

    <template #extra>
      <template v-if="status.xray.state !== 'error'">
        <a-badge status="processing" :class="['xray-processing-animation', badgeAnimationClass(status.xray.color)]"
          :text="stateText" :color="status.xray.color" />
      </template>
      <template v-else>
        <a-popover>
          <template #title>
            <a-row type="flex" align="middle" justify="space-between">
              <a-col><span>{{ t('pages.index.xrayStatusError') }}</span></a-col>
              <a-col>
                <BarsOutlined class="cursor-pointer" @click="$emit('open-logs')" />
              </a-col>
            </a-row>
          </template>
          <template #content>
            <span v-for="(line, i) in (status.xray.errorMsg || '').split('\n')" :key="i" class="error-line">
              {{ line }}
            </span>
          </template>
          <a-badge status="processing" :text="stateText" :color="status.xray.color"
            :class="['xray-processing-animation', 'xray-error-animation']" />
        </a-popover>
      </template>
    </template>

    <template #actions>
      <a-space v-if="ipLimitEnable" direction="horizontal" class="action" @click="$emit('open-xray-logs')">
        <BarsOutlined />
        <span v-if="!isMobile">{{ t('pages.index.logs') }}</span>
      </a-space>
      <a-space direction="horizontal" class="action" @click="$emit('stop-xray')">
        <PoweroffOutlined />
        <span v-if="!isMobile">{{ t('pages.index.stopXray') }}</span>
      </a-space>
      <a-space direction="horizontal" class="action" @click="$emit('restart-xray')">
        <ReloadOutlined />
        <span v-if="!isMobile">{{ t('pages.index.restartXray') }}</span>
      </a-space>
      <a-space direction="horizontal" class="action" @click="$emit('open-version-switch')">
        <ToolOutlined />
        <span v-if="!isMobile">
          {{ status.xray.version && status.xray.version !== 'Unknown'
            ? `v${status.xray.version}`
            : t('pages.index.xraySwitch') }}
        </span>
      </a-space>
    </template>
  </a-card>
</template>

<style scoped>
.action {
  cursor: pointer;
  justify-content: center;
}

.error-line {
  display: block;
  max-width: 400px;
  white-space: pre-wrap;
}

.cursor-pointer {
  cursor: pointer;
}
</style>

<style>
/* Legacy xray-*-animation classes — they need to be global so they
 * pierce the AD-Vue badge's internal DOM (.ant-badge-status-*). */
.xray-processing-animation .ant-badge-status-dot {
  animation: xray-pulse 1.2s linear infinite;
}

.xray-running-animation .ant-badge-status-processing::after {
  border-color: #1677ff;
}

.xray-stop-animation .ant-badge-status-processing::after {
  border-color: #fa8c16;
}

.xray-error-animation .ant-badge-status-processing::after {
  border-color: #f5222d;
}

@keyframes xray-pulse {

  0%,
  50%,
  100% {
    transform: scale(1);
    opacity: 1;
  }

  10% {
    transform: scale(1.5);
    opacity: 0.2;
  }
}
</style>
