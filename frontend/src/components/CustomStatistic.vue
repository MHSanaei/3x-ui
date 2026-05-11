<script setup>
defineProps({
  title: { type: String, default: '' },
  value: { type: [String, Number], default: '' },
});
</script>

<template>
  <a-statistic :title="title" :value="value">
    <template #prefix>
      <slot name="prefix" />
    </template>
    <template #suffix>
      <slot name="suffix" />
    </template>
  </a-statistic>
</template>

<style scoped>
:deep(.ant-statistic-content) {
  font-size: 16px;
}

:global(body.dark .ant-statistic-content) {
  color: var(--dark-color-text-primary);
}

:global(body.dark .ant-statistic-title) {
  color: rgba(255, 255, 255, 0.55);
}
</style>
