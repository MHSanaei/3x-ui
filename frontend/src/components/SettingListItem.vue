<script setup>
import { computed } from 'vue';

const props = defineProps({
  paddings: {
    type: String,
    default: 'default',
    validator: (value) => ['small', 'default'].includes(value),
  },
});

const padding = computed(() =>
  props.paddings === 'small' ? '10px 20px !important' : '20px !important',
);
</script>

<template>
  <a-list-item :style="{ padding }">
    <a-row :gutter="[8, 16]">
      <a-col :xs="24" :lg="12">
        <a-list-item-meta>
          <template #title>
            <slot name="title" />
          </template>
          <template #description>
            <slot name="description" />
          </template>
        </a-list-item-meta>
      </a-col>
      <a-col :xs="24" :lg="12">
        <slot name="control" />
      </a-col>
    </a-row>
  </a-list-item>
</template>
