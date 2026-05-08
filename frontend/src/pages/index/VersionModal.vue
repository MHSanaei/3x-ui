<script setup>
import { ref, watch } from 'vue';
import { Modal } from 'ant-design-vue';
import { ReloadOutlined } from '@ant-design/icons-vue';
import { HttpUtil } from '@/utils';
import CustomGeoSection from './CustomGeoSection.vue';

const props = defineProps({
  open: { type: Boolean, default: false },
  status: { type: Object, required: true },
});

const emit = defineEmits(['update:open', 'busy']);

const activeKey = ref('1');
const versions = ref([]);
const loading = ref(false);

// Geofiles list is hardcoded in the legacy panel — same set of files
// served from /panel/api/server/updateGeofile/{name}.
const GEOFILES = ['geosite.dat', 'geoip.dat', 'geosite_IR.dat', 'geoip_IR.dat', 'geosite_RU.dat', 'geoip_RU.dat'];

async function fetchVersions() {
  loading.value = true;
  try {
    const msg = await HttpUtil.get('/panel/api/server/getXrayVersion');
    if (msg?.success) versions.value = msg.obj || [];
  } finally {
    loading.value = false;
  }
}

function close() {
  emit('update:open', false);
}

function switchXrayVersion(version) {
  Modal.confirm({
    title: 'Switch xray version',
    content: `Are you sure you want to install ${version}? This will restart xray.`,
    okText: 'Confirm',
    cancelText: 'Cancel',
    onOk: async () => {
      close();
      emit('busy', { busy: true, tip: `Installing ${version}…` });
      try {
        await HttpUtil.post(`/panel/api/server/installXray/${version}`);
      } finally {
        emit('busy', { busy: false });
      }
    },
  });
}

function updateGeofile(fileName) {
  const isSingle = !!fileName;
  Modal.confirm({
    title: 'Update geofile',
    content: isSingle
      ? `Update ${fileName}? Xray will restart after the file is replaced.`
      : 'Update all geofiles? Xray will restart after the files are replaced.',
    okText: 'Confirm',
    cancelText: 'Cancel',
    onOk: async () => {
      close();
      emit('busy', { busy: true, tip: 'Updating geofiles…' });
      const url = isSingle
        ? `/panel/api/server/updateGeofile/${fileName}`
        : '/panel/api/server/updateGeofile';
      try {
        await HttpUtil.post(url);
      } finally {
        emit('busy', { busy: false });
      }
    },
  });
}

watch(() => props.open, (next) => { if (next) fetchVersions(); });
</script>

<template>
  <a-modal :open="open" title="Xray updates" :closable="true" :footer="null" @cancel="close">
    <a-spin :spinning="loading">
      <a-collapse v-model:active-key="activeKey" accordion>
        <a-collapse-panel key="1" header="Xray">
          <a-alert
            type="warning"
            class="mb-12"
            message="Click a version to install it. Xray will restart automatically."
            show-icon
          />
          <a-list bordered class="version-list">
            <a-list-item v-for="(version, index) in versions" :key="version" class="version-list-item">
              <a-tag :color="index % 2 === 0 ? 'purple' : 'green'">{{ version }}</a-tag>
              <a-radio
                :checked="version === `v${status?.xray?.version}`"
                @click="switchXrayVersion(version)"
              />
            </a-list-item>
          </a-list>
        </a-collapse-panel>

        <a-collapse-panel key="2" header="Geofiles">
          <a-list bordered class="version-list">
            <a-list-item v-for="(file, index) in GEOFILES" :key="file" class="version-list-item">
              <a-tag :color="index % 2 === 0 ? 'purple' : 'green'">{{ file }}</a-tag>
              <a-tooltip title="Update this file">
                <ReloadOutlined class="reload-icon" @click="updateGeofile(file)" />
              </a-tooltip>
            </a-list-item>
          </a-list>
          <div class="actions-row">
            <a-button @click="updateGeofile('')">Update all</a-button>
          </div>
        </a-collapse-panel>

        <a-collapse-panel key="3" header="Custom geo">
          <CustomGeoSection :active="activeKey === '3'" />
        </a-collapse-panel>
      </a-collapse>
    </a-spin>
  </a-modal>
</template>

<style scoped>
.mb-12 { margin-bottom: 12px; }
.version-list { width: 100%; }
.version-list-item { display: flex; justify-content: space-between; align-items: center; }

.reload-icon {
  cursor: pointer;
  font-size: 16px;
  margin-right: 8px;
}

.actions-row {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}
</style>
