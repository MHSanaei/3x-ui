<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { AreaChartOutlined } from '@ant-design/icons-vue';

import { CPUFormatter, SizeFormatter } from '@/utils';

const { t } = useI18n();

const props = defineProps({
  status: { type: Object, required: true },
  isMobile: { type: Boolean, default: false },
});

// AD-Vue's default 120px dashboard renders the percent text at ~36px
// which dwarfs the rest of the card. 70 (60 on mobile) plus the
// :deep(.ant-progress-text) override below keep the gauges compact.
const gaugeSize = computed(() => (props.isMobile ? 60 : 70));

// AD-Vue's default unfinished trail (rgba(0,0,0,0.06) /
// rgba(255,255,255,0.08)) is invisible against the light card; a
// neutral mid-gray reads on both themes.
const trailColor = 'rgba(128, 128, 128, 0.25)';
</script>

<template>
  <a-card hoverable>
    <a-row :gutter="[0, isMobile ? 16 : 0]">
      <!-- CPU + Memory -->
      <a-col :xs="24" :md="12">
        <a-row>
          <a-col :span="12" class="text-center">
            <a-progress type="dashboard" status="normal" :stroke-color="status.cpu.color" :trail-color="trailColor"
              :percent="status.cpu.percent" :width="gaugeSize" />
            <div>
              <b>{{ t('pages.index.cpu') }}:</b> {{ CPUFormatter.cpuCoreFormat(status.cpuCores) }}
              <a-tooltip>
                <template #title>
                  <div><b>{{ t('pages.index.logicalProcessors') }}:</b> {{ status.logicalPro }}</div>
                  <div><b>{{ t('pages.index.frequency') }}:</b> {{ CPUFormatter.cpuSpeedFormat(status.cpuSpeedMhz) }}
                  </div>
                </template>
                <AreaChartOutlined />
              </a-tooltip>
            </div>
          </a-col>

          <a-col :span="12" class="text-center">
            <a-progress type="dashboard" status="normal" :stroke-color="status.mem.color" :trail-color="trailColor"
              :percent="status.mem.percent" :width="gaugeSize" />
            <div>
              <b>{{ t('pages.index.memory') }}:</b> {{ SizeFormatter.sizeFormat(status.mem.current) }} /
              {{ SizeFormatter.sizeFormat(status.mem.total) }}
            </div>
          </a-col>
        </a-row>
      </a-col>

      <!-- Swap + Disk -->
      <a-col :xs="24" :md="12">
        <a-row>
          <a-col :span="12" class="text-center">
            <a-progress type="dashboard" status="normal" :stroke-color="status.swap.color" :trail-color="trailColor"
              :percent="status.swap.percent" :width="gaugeSize" />
            <div>
              <b>{{ t('pages.index.swap') }}:</b> {{ SizeFormatter.sizeFormat(status.swap.current) }} /
              {{ SizeFormatter.sizeFormat(status.swap.total) }}
            </div>
          </a-col>

          <a-col :span="12" class="text-center">
            <a-progress type="dashboard" status="normal" :stroke-color="status.disk.color" :trail-color="trailColor"
              :percent="status.disk.percent" :width="gaugeSize" />
            <div>
              <b>{{ t('pages.index.storage') }}:</b> {{ SizeFormatter.sizeFormat(status.disk.current) }} /
              {{ SizeFormatter.sizeFormat(status.disk.total) }}
            </div>
          </a-col>
        </a-row>
      </a-col>
    </a-row>
  </a-card>
</template>

<style scoped>
.text-center {
  text-align: center;
}

/* Pin the percent number to a label-sized 14px — AD-Vue scales it
 * from the SVG's intrinsic size, so :width alone leaves it too big. */
:deep(.ant-progress-text) {
  font-size: 14px !important;
  font-weight: 500;
}
</style>
