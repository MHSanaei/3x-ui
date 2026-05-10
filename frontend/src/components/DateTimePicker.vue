<script setup>
import { computed, defineAsyncComponent } from 'vue';
import dayjs from 'dayjs';
import { useDatepicker } from '@/composables/useDatepicker.js';

const PersianDatePicker = defineAsyncComponent(() => import('vue3-persian-datetime-picker'));

const props = defineProps({
  value: { type: [Object, null], default: null },
  showTime: { type: Boolean, default: true },
  format: { type: String, default: 'YYYY-MM-DD HH:mm:ss' },
  placeholder: { type: String, default: '' },
  disabled: { type: Boolean, default: false },
});

const emit = defineEmits(['update:value']);

const { datepicker } = useDatepicker();
const isJalali = computed(() => datepicker.value === 'jalalian');

const ISO_FORMAT = 'YYYY-MM-DD HH:mm:ss';

// Persian picker's display format — `j…` tokens come from moment-jalaali
// and render Jalali year/month/day.
const persianDisplayFormat = computed(() =>
  props.showTime ? 'jYYYY/jMM/jDD HH:mm:ss' : 'jYYYY/jMM/jDD',
);

// Persian picker stores the date as a Gregorian string in the format
// it was given via `format`. We normalize on `YYYY-MM-DD HH:mm:ss` so
// dayjs(...) round-trips cleanly.
const stringValue = computed({
  get() {
    const v = props.value;
    if (!v) return '';
    return dayjs.isDayjs(v) ? v.format(ISO_FORMAT) : dayjs(v).format(ISO_FORMAT);
  },
  set(next) {
    if (!next) {
      emit('update:value', null);
      return;
    }
    const parsed = dayjs(next, ISO_FORMAT);
    emit('update:value', parsed.isValid() ? parsed : null);
  },
});

function onAntChange(next) {
  emit('update:value', next || null);
}
</script>

<template>
  <PersianDatePicker v-if="isJalali" v-model="stringValue" :format="ISO_FORMAT" :display-format="persianDisplayFormat"
    :placeholder="placeholder" :disabled="disabled" color="#1677ff" auto-submit append-to="body"
    input-class="ant-input persian-datepicker-input" class="jalali-datepicker" />
  <a-date-picker v-else :value="value" :show-time="showTime ? { format: 'HH:mm:ss' } : false" :format="format"
    :placeholder="placeholder" :disabled="disabled" :style="{ width: '100%' }" @update:value="onAntChange" />
</template>

<style scoped>
.jalali-datepicker {
  width: 100%;
}
</style>

<!-- Theme overrides for the picker. AD-Vue 4 doesn't expose CSS variables
     by default (its tokens live in JS), so we hardcode hexes per theme
     class — `body.dark` for the navy theme, `[data-theme="ultra-dark"]`
     for the neutral ultra-dark variant. The popup stays inside the
     wrapper's subtree (no teleport) so global selectors reach it cleanly. -->
<style>
/* ===== Light (default) =================================================== */

.persian-datepicker-input {
  width: 100%;
  box-sizing: border-box;
  padding: 4px 11px;
  font-size: 14px;
  border: 1px solid #d9d9d9;
  border-radius: 6px;
  background: #fff;
  color: rgba(0, 0, 0, 0.88);
  transition: border-color 0.2s, box-shadow 0.2s;
}

.persian-datepicker-input:hover {
  border-color: #4096ff;
}

.persian-datepicker-input:focus {
  border-color: #1677ff;
  box-shadow: 0 0 0 2px rgba(22, 119, 255, 0.1);
  outline: none;
}

/* Light theme keeps the picker's brand-blue calendar button (set via
 * inline style on .vpd-icon-btn) — only its border + corner radius are
 * normalized so it sits flush with the input. Dark/ultra-dark themes
 * below override the inline blue so the control matches the form. */
.vpd-main .vpd-icon-btn {
  color: #fff;
  border: 1px solid transparent;
  border-radius: 6px 0 0 6px;
}

/* Match the input's left edge (no rounded left, no double border at the
 * seam) so it sits flush against the icon-btn. */
.persian-datepicker-input {
  border-top-left-radius: 0;
  border-bottom-left-radius: 0;
}

.vpd-main .vpd-clear-btn {
  color: rgba(0, 0, 0, 0.45);
  background: transparent;
}

/* Width is exactly 316px so the 7-day grid (7 × 40px + 36px padding)
 * fits flush. Don't add `border` here — box-sizing: border-box would
 * eat 2px from the content width and the 7th day-cell of each row
 * wraps. Use box-shadow + a wider radius for the visual edge instead. */
.vpd-wrapper .vpd-content {
  background: #fff;
  color: rgba(0, 0, 0, 0.88);
  box-shadow: 0 6px 16px 0 rgba(0, 0, 0, 0.08),
    0 3px 6px -4px rgba(0, 0, 0, 0.12),
    0 9px 28px 8px rgba(0, 0, 0, 0.05);
  border-radius: 8px;
  overflow: hidden;
}

.vpd-wrapper .vpd-header {
  background: #1677ff;
  color: #fff;
  border-radius: 8px 8px 0 0;
}

.vpd-wrapper .vpd-header .vpd-year-label,
.vpd-wrapper .vpd-header .vpd-date,
.vpd-wrapper .vpd-header .vpd-locales li {
  color: #fff;
}

.vpd-wrapper .vpd-body {
  background: #fff;
  color: rgba(0, 0, 0, 0.88);
}

.vpd-wrapper .vpd-body .vpd-month-label,
.vpd-wrapper .vpd-body .vpd-month-label>span {
  color: rgba(0, 0, 0, 0.88);
}

.vpd-wrapper .vpd-body .vpd-week,
.vpd-wrapper .vpd-body .vpd-weekday {
  color: rgba(0, 0, 0, 0.55);
}

.vpd-wrapper .vpd-body .vpd-controls .vpd-next,
.vpd-wrapper .vpd-body .vpd-controls .vpd-prev {
  color: rgba(0, 0, 0, 0.65);
}

/* The picker's <arrow> component renders an inline SVG with a hardcoded
 * `fill="#000"` attribute. Override the path fill via CSS so the arrow
 * is visible in every theme. */
.vpd-wrapper .vpd-next svg path,
.vpd-wrapper .vpd-prev svg path {
  fill: rgba(0, 0, 0, 0.65);
}

.vpd-wrapper .vpd-body .vpd-controls .vpd-next:hover svg path,
.vpd-wrapper .vpd-body .vpd-controls .vpd-prev:hover svg path {
  fill: #1677ff;
}

/* The picker paints disabled days as `darken(#fff, 20%)` (~#cccccc) which
 * is invisible on white and dark themes alike. Reset the day text color
 * across all states so days are always readable. */
.vpd-wrapper .vpd-day,
.vpd-wrapper .vpd-day .vpd-day-text {
  color: rgba(0, 0, 0, 0.88) !important;
}

.vpd-wrapper .vpd-day[disabled='true'],
.vpd-wrapper .vpd-day[disabled='true'] .vpd-day-text {
  color: rgba(0, 0, 0, 0.25) !important;
}

.vpd-wrapper .vpd-day:not([disabled='true']):hover .vpd-day-text,
.vpd-wrapper .vpd-day.vpd-selected .vpd-day-text {
  color: #fff !important;
}

.vpd-wrapper .vpd-actions button {
  color: rgba(0, 0, 0, 0.88);
  background: transparent;
}

.vpd-wrapper .vpd-actions button:hover {
  background: rgba(0, 0, 0, 0.04);
  color: #1677ff;
}

.vpd-wrapper .vpd-addon-list,
.vpd-wrapper .vpd-addon-list-content {
  background: #fff;
  color: rgba(0, 0, 0, 0.88);
}

.vpd-wrapper .vpd-addon-list-item {
  color: rgba(0, 0, 0, 0.88);
  border-color: #fff;
}

.vpd-wrapper .vpd-addon-list-item.vpd-selected,
.vpd-wrapper .vpd-addon-list-item:hover {
  background: rgba(0, 0, 0, 0.04);
}

.vpd-wrapper .vpd-close-addon {
  color: rgba(0, 0, 0, 0.65);
  background: rgba(0, 0, 0, 0.06);
}

/* ===== Dark (navy) ======================================================= */

body.dark .persian-datepicker-input {
  background: #142340;
  border-color: #1f3358;
  color: rgba(255, 255, 255, 0.88);
}

body.dark .persian-datepicker-input:hover {
  border-color: #4096ff;
}

body.dark .persian-datepicker-input:focus {
  border-color: #1677ff;
  box-shadow: 0 0 0 2px rgba(22, 119, 255, 0.18);
}

body.dark .vpd-main .vpd-icon-btn {
  background: rgba(255, 255, 255, 0.04) !important;
  border: 1px solid #1f3358 !important;
  border-right: none !important;
  border-radius: 6px 0 0 6px !important;
  color: rgba(255, 255, 255, 0.75) !important;
}

body.dark .vpd-wrapper .vpd-content {
  background: #1a2c4d;
  color: rgba(255, 255, 255, 0.88);
  box-shadow: 0 6px 16px 0 rgba(0, 0, 0, 0.32),
    0 3px 6px -4px rgba(0, 0, 0, 0.48),
    0 9px 28px 8px rgba(0, 0, 0, 0.2);
}

body.dark .vpd-wrapper .vpd-body {
  background: #1a2c4d;
  color: rgba(255, 255, 255, 0.88);
}

body.dark .vpd-wrapper .vpd-body .vpd-month-label,
body.dark .vpd-wrapper .vpd-body .vpd-month-label>span {
  color: rgba(255, 255, 255, 0.88);
}

body.dark .vpd-wrapper .vpd-body .vpd-week,
body.dark .vpd-wrapper .vpd-body .vpd-weekday {
  color: rgba(255, 255, 255, 0.55);
}

body.dark .vpd-wrapper .vpd-body .vpd-controls .vpd-next,
body.dark .vpd-wrapper .vpd-body .vpd-controls .vpd-prev {
  color: rgba(255, 255, 255, 0.65);
}

body.dark .vpd-wrapper .vpd-next svg path,
body.dark .vpd-wrapper .vpd-prev svg path {
  fill: rgba(255, 255, 255, 0.75);
}

body.dark .vpd-wrapper .vpd-body .vpd-controls .vpd-next:hover svg path,
body.dark .vpd-wrapper .vpd-body .vpd-controls .vpd-prev:hover svg path {
  fill: #4096ff;
}

body.dark .vpd-wrapper .vpd-day,
body.dark .vpd-wrapper .vpd-day .vpd-day-text {
  color: rgba(255, 255, 255, 0.88) !important;
}

body.dark .vpd-wrapper .vpd-day[disabled='true'],
body.dark .vpd-wrapper .vpd-day[disabled='true'] .vpd-day-text {
  color: rgba(255, 255, 255, 0.25) !important;
}

body.dark .vpd-wrapper .vpd-actions button {
  color: rgba(255, 255, 255, 0.88);
}

body.dark .vpd-wrapper .vpd-actions button:hover {
  background: rgba(255, 255, 255, 0.06);
}

body.dark .vpd-wrapper .vpd-addon-list,
body.dark .vpd-wrapper .vpd-addon-list-content {
  background: #1a2c4d;
  color: rgba(255, 255, 255, 0.88);
}

body.dark .vpd-wrapper .vpd-addon-list-item {
  color: rgba(255, 255, 255, 0.88);
  border-color: transparent;
}

body.dark .vpd-wrapper .vpd-addon-list-item.vpd-selected,
body.dark .vpd-wrapper .vpd-addon-list-item:hover {
  background: rgba(255, 255, 255, 0.06);
}

body.dark .vpd-wrapper .vpd-close-addon {
  color: rgba(255, 255, 255, 0.65);
  background: rgba(255, 255, 255, 0.08);
}

/* ===== Ultra-dark (neutral black) ======================================= */

html[data-theme='ultra-dark'] .persian-datepicker-input {
  background: #0a0a0a;
  border-color: #303030;
  color: rgba(255, 255, 255, 0.88);
}

html[data-theme='ultra-dark'] .vpd-main .vpd-icon-btn {
  background: rgba(255, 255, 255, 0.04) !important;
  border: 1px solid #303030 !important;
  border-right: none !important;
  border-radius: 6px 0 0 6px !important;
  color: rgba(255, 255, 255, 0.75) !important;
}

html[data-theme='ultra-dark'] .vpd-wrapper .vpd-content {
  background: #141414;
  color: rgba(255, 255, 255, 0.88);
}

html[data-theme='ultra-dark'] .vpd-wrapper .vpd-body {
  background: #141414;
}

html[data-theme='ultra-dark'] .vpd-wrapper .vpd-addon-list,
html[data-theme='ultra-dark'] .vpd-wrapper .vpd-addon-list-content {
  background: #141414;
}
</style>
