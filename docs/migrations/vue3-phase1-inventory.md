# Vue 3 + Ant Design Vue 4 Migration — Phase 1 Inventory

Branch: `vue3-migration`
Source state: Vue 2 + Ant Design Vue 1.7.8, no build step, Go template-driven.

## Scope

- **69 HTML templates**, ~17,650 total lines
- **Largest pages:**
  - `web/html/xray.html` — 2,360 lines
  - `web/html/index.html` — ~1,700 lines
  - `web/html/settings.html` — 720 lines
  - `web/html/settings/xray/outbounds.html` — 263 lines

## Vue 2 → Vue 3 breakage surface

| Pattern | Count | Files | Severity | Notes |
|---|---:|---:|---|---|
| `{{ x \| filter }}` | **0** | 0 | ✅ none | Filters removed in Vue 3 — but we don't use any. |
| `<template slot="X">` | 233 | 36 | medium | Rewrite to `<template #X>`. Mechanical. |
| `slot-scope="X"` (incl. above) | 275 total | 40 | medium | Rewrite to `v-slot:name="X"`. Mechanical. |
| `scopedSlots: { ... }` (in JS column defs) | 49 | 4 | medium | Vue 3 removed `scopedSlots`. All slots are now scoped. Replace with `slots: { ... }` or rely on template slot binding only. |
| `new Vue({...})` mounts | ~30+ | 30 | medium | Replace with `createApp({...}).mount('#id')`. Each page mounts its own Vue instance. |
| `Vue.use / Vue.component / Vue.prototype` | <49 (after subtracting mounts) | various | medium | Replace with `app.use / app.component / app.config.globalProperties`. |
| `$listeners` / `$on` / `$off` / `$once` / `$children` | 4 | 4 | low | Mostly inside `aTableSortable.html`. `$listeners` merged into `$attrs`. Event-bus methods removed — need an explicit emitter (mitt, or component refs). |
| `inline-template` / `functional` | 1 | 1 | trivial | One occurrence. |
| `.sync` modifier | 0 | 0 | ✅ none | Removed in Vue 3, replaced by `v-model:propName`. We don't use it. |
| `v-model` | 358 | 36 | **high** | Default `v-model` on custom components changed (`value` → `modelValue`, `input` event → `update:modelValue`). Most of these target AD-Vue components, which AD-Vue 4 already adapts to internally — but components we wrote ourselves (e.g. `aClientTable`, `aTableSortable`) need updates. |
| `Vue.set` / `Vue.delete` / `Vue.observable` | 0 | 0 | ✅ none | Replaced by `reactive()`. Not needed. |
| `transition` class names (`-enter`, `-leave`) | 1 (in `qrcode_modal.html`) | 1 | low | Renamed in Vue 3: `*-enter` → `*-enter-from`, `*-leave-to` stays, etc. |
| Key modifiers using `keyCode` (`@keyup.13`) | 0 | 0 | ✅ none | Number key codes removed in Vue 3. We don't use them. |
| `ref="..."` attrs | 21 | 9 | low | Behavior unchanged for Options API — refs still work the same. |
| `.native` event modifier | 0 | 0 | ✅ none | Removed in Vue 3 (events on components are no longer "fake" by default). |

## Ant Design Vue 1.x → 4.x breakage surface

Total `<a-*>` component instances: **3,145 across 63 files**. This is the bulk of the migration cost.

The most-used components (rough estimate from grep):

- `a-input`, `a-select`, `a-button` — heavily used; props mostly stable, but `<a-select-option>` slot syntax changed
- `a-form` + `a-form-item` — Form API was substantially redesigned in AD-Vue 3+. We use it lightly (`layout="vertical"`, `label-col`, `wrapper-col`); migration is mechanical but every form needs touching.
- `a-table` — column definitions with `scopedSlots` (49 instances) need to become `slots: { customRender: 'name' }` or use template slot binding directly.
- `a-modal` — **`v-model` on a-modal changed**: `v-model="visible"` → `v-model:open="visible"` (or `v-model:visible` depending on version). Every modal needs updating.
- `a-icon` — **removed as a generic component in AD-Vue 4.** Each icon must be imported individually from `@ant-design/icons-vue`. We use ~233 `a-icon` references — mostly via `type="..."` attribute. Likely the single highest-friction change.
- `a-tooltip`, `a-popover`, `a-drawer` — slot-based titles (`<template slot="title">` → `<template #title>`).
- `a-collapse` + `a-collapse-panel` — header slot syntax changes.
- `a-tabs` + `a-tab-pane` — `tab` slot rewrite, possibly renamed.
- `a-space` — exists in AD-Vue 4, props stable.
- `a-tag` — stable.

## Key custom code

- `web/assets/js/util/index.js` — utilities (HttpUtil, ObjectUtil, ClipboardManager, SizeFormatter, etc.). Framework-agnostic. **Trivial migration** (no Vue dependency).
- `web/assets/js/axios-init.js` — axios setup. **Trivial migration**.
- `web/assets/js/websocket.js` — websocket client. **Trivial migration** (we recently refactored this).
- `web/assets/js/model/{inbound,outbound,dbinbound,setting,reality_targets}.js` — domain model classes. Plain JS. **Trivial migration**.
- `web/assets/js/subscription.js` — subscription page logic.
- Custom components in `web/html/component/`:
  - `aClientTable.html` — non-trivial; uses scoped slots and v-model
  - `aSidebar.html` — sidebar navigation
  - `aThemeSwitch.html` — theme picker
  - `aPersianDatepicker.html` — wraps a third-party datepicker
  - `aTableSortable.html` — uses `$listeners`, `$on` — needs explicit refactor
  - `aSettingListItem.html`, `aCustomStatistic.html` — small wrappers
  - These are the **only places using the deprecated event-bus APIs** (4 occurrences).

## Server-side coupling

The Go layer interpolates translations directly into templates via `{{ i18n "key" }}`. After migration we have two options:

- **Keep Go-side i18n.** Vite builds .html partials that still get processed by Go's `template` package. Means `<script type="module">` entrypoints reference build artifacts but markup is still server-rendered. Pro: smallest change. Con: every page change forces a Vite rebuild *and* a Go restart.
- **Move i18n to client side.** Export the translation TOML files as JSON, ship as static assets, use `vue-i18n`. Pro: cleaner client/server split, hot reload during dev. Con: more change, every i18n key reference in templates must be rewritten.

We will defer this decision to Phase 7. For Phases 2–4 we keep the Go-side approach.

## Risk-ranked migration order

Order chosen so that breakage is contained and we always have a working panel:

1. **Phase 2 — Toolchain.** Vite scaffold; Go binary embeds `dist/` via `embed.FS`. New build runs in CI alongside existing static assets; legacy continues to work.
2. **Phase 3 — Utils.** Migrate framework-agnostic JS first. Zero Vue dependency, zero risk.
3. **Phase 4 — `login.html`.** Smallest page with state. Hits every Vue 2→3 syntax change and every AD-Vue 1→4 component change at small scale. Becomes the template the rest follow.
4. **Phase 5 — Medium pages and modals.** `index.html`, `settings.html`, all modals. ~30 templates of 200-1000 lines each.
5. **Phase 6 — `xray.html`.** The 2,360-line page with the inbound/outbound editors. Highest regression risk — will likely break and need fixing in the QA pass.
6. **Phase 7 — i18n decision.**
7. **Phase 8 — Regression pass + delete legacy templates + cut release.**

## Numbers to remember

- **6–8 weeks** of focused work, single developer
- **63 HTML files** to touch
- **3,145 AD-Vue component instances** to validate
- **One** assumption that needs confirming with the user: build step OK (yes — confirmed by choice of Vite)

## Confirmed user decisions

- ✅ Migrate to Vue 3 + Ant Design Vue 4
- ✅ Introduce Vite build step (npm acceptable)
- ✅ Work on a long-running `vue3-migration` branch
- ⏸ i18n strategy — to be decided in Phase 7
