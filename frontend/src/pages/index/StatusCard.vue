<script setup>
import { AreaChartOutlined, HistoryOutlined } from '@ant-design/icons-vue';

import { CPUFormatter, SizeFormatter } from '@/utils';

defineProps({
  status: { type: Object, required: true },
  isMobile: { type: Boolean, default: false },
});

defineEmits(['open-cpu-history']);
</script>

<template>
  <a-card hoverable>
    <a-row :gutter="[0, isMobile ? 16 : 0]">
      <!-- CPU + Memory -->
      <a-col :sm="24" :md="12">
        <a-row>
          <a-col :span="12" class="text-center">
            <a-progress
              type="dashboard"
              status="normal"
              :stroke-color="status.cpu.color"
              :percent="status.cpu.percent"
            />
            <div>
              <b>CPU:</b> {{ CPUFormatter.cpuCoreFormat(status.cpuCores) }}
              <a-tooltip>
                <template #title>
                  <div><b>Logical processors:</b> {{ status.logicalPro }}</div>
                  <div><b>Frequency:</b> {{ CPUFormatter.cpuSpeedFormat(status.cpuSpeedMhz) }}</div>
                </template>
                <AreaChartOutlined />
              </a-tooltip>
              <a-tooltip>
                <template #title>CPU history</template>
                <a-button size="small" shape="circle" class="ml-8" @click="$emit('open-cpu-history')">
                  <template #icon><HistoryOutlined /></template>
                </a-button>
              </a-tooltip>
            </div>
          </a-col>

          <a-col :span="12" class="text-center">
            <a-progress
              type="dashboard"
              status="normal"
              :stroke-color="status.mem.color"
              :percent="status.mem.percent"
            />
            <div>
              <b>Memory:</b> {{ SizeFormatter.sizeFormat(status.mem.current) }} /
              {{ SizeFormatter.sizeFormat(status.mem.total) }}
            </div>
          </a-col>
        </a-row>
      </a-col>

      <!-- Swap + Disk -->
      <a-col :sm="24" :md="12">
        <a-row>
          <a-col :span="12" class="text-center">
            <a-progress
              type="dashboard"
              status="normal"
              :stroke-color="status.swap.color"
              :percent="status.swap.percent"
            />
            <div>
              <b>Swap:</b> {{ SizeFormatter.sizeFormat(status.swap.current) }} /
              {{ SizeFormatter.sizeFormat(status.swap.total) }}
            </div>
          </a-col>

          <a-col :span="12" class="text-center">
            <a-progress
              type="dashboard"
              status="normal"
              :stroke-color="status.disk.color"
              :percent="status.disk.percent"
            />
            <div>
              <b>Storage:</b> {{ SizeFormatter.sizeFormat(status.disk.current) }} /
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
.ml-8 {
  margin-left: 8px;
}
</style>
