<script setup>
// Inline ∞ SVG. The Unicode infinity character (U+221E) renders as an
// "m"-shaped glyph in some system fonts (Windows Segoe UI in particular),
// so the inbound list and client row table use this SVG instead. The
// path matches what the legacy panel embedded.
defineProps({
  width: { type: [String, Number], default: 14 },
  height: { type: [String, Number], default: 10 },
});
</script>

<template>
  <svg :width="width" :height="height" viewBox="0 0 640 512" fill="currentColor" aria-hidden="true"
    style="vertical-align: -1px; display: inline-block;">
    <path
      d="M484.4 96C407 96 349.2 164.1 320 208.5C290.8 164.1 233 96 155.6 96C69.75 96 0 167.8 0 256s69.75 160 155.6 160C233.1 416 290.8 347.9 320 303.5C349.2 347.9 407 416 484.4 416C570.3 416 640 344.2 640 256S570.3 96 484.4 96zM155.6 368C96.25 368 48 317.8 48 256s48.25-112 107.6-112c67.75 0 120.5 82.25 137.1 112C276 285.8 223.4 368 155.6 368zM484.4 368c-67.75 0-120.5-82.25-137.1-112C364 226.2 416.6 144 484.4 144C543.8 144 592 194.2 592 256S543.8 368 484.4 368z" />
  </svg>
</template>
