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

## Phase progress

- ✅ Phase 1 — inventory (this document)
- ✅ Phase 2 — Vite + Vue 3 + AD-Vue 4 scaffold under `frontend/`
- ✅ Phase 3 — utils + models + websocket ported as ES modules
- ✅ Phase 4 — first real page (login.html) ported
- ⏳ Phase 5 — medium pages + modals
  - ✅ 5a — theme system (composable + ThemeSwitch / ThemeSwitchLogin); wired into login
  - ⏳ 5b — remaining custom components

### Phase 5a notes

- `aThemeSwitch.html` had two near-identical components (full menu version + login popover version) plus a `themeSwitcher` global object. Refactored into:
  - `composables/useTheme.js` — single reactive `theme` state, `toggleTheme/toggleUltra`, and a `pauseAnimationsUntilLeave` helper. Boot side-effects (apply stored theme + persist on change) run via `watchEffect`. Importing the module is enough to apply the right theme before mount.
  - `components/ThemeSwitch.vue` — full menu version (used in the main panel sidebar).
  - `components/ThemeSwitchLogin.vue` — login popover version.
- AD-Vue 4 dropped `<a-icon>`. The `BulbFilled` / `BulbOutlined` swap is done via `<component :is="BulbIcon">`.
- `Vue.component('a-theme-switch', { ... })` global registration → per-page imports.
- `this.$message.config(...)` (Vue 2 instance method) → `message.config(...)` from `ant-design-vue`, called once at app boot in `login.js`.
- vue-i18n bumped 10 → 11.1.4 (npm warned that v9/v10 are EOL).
- Vite 8.0.11 build verified — `npm run build` succeeds, outputs `web/dist/login.html` + chunked JS/CSS. AD-Vue with `app.use(Antd)` produces a 1.5 MB chunk; we'll switch to per-component imports in a later cleanup pass.

**Known gap:** the legacy `web/assets/css/custom.min.css` styles `body.dark` / `body.light` / `[data-theme="ultra-dark"]`. The new login page doesn't import that CSS, so toggling theme switches AD-Vue's own components but not the panel chrome (e.g. card backgrounds). The composable still toggles the body class so behavior is correct — visual fidelity is restored when we either port custom.css to the new build or import it directly.

### Phase 4 notes

- Vite 6 → Vite 8.0.11 (released March 2026). Requires Node 20.19+ or 22.12+. `@vitejs/plugin-vue` bumped to ^6.0.6 which lists vite ^8 as a peer.
- Multi-page Vite: each migrated page = its own entry. `frontend/login.html` registered alongside `frontend/index.html` in `rollupOptions.input`. Same approach for the rest of the migration.
- New page lives at `frontend/src/pages/login/LoginPage.vue` with a thin entrypoint at `frontend/src/login.js` that calls `setupAxios()`, installs Antd, and mounts.
- **Three legacy features deferred** to keep Phase 4 small:
  - **i18n** — strings hardcoded in English. Phase 7 wires up vue-i18n with the existing TOML files.
  - **Theme switcher** — `<a-theme-switch-login>` was a custom component referencing a global `themeSwitcher`. Deferred to Phase 5 with the rest of the shared components.
  - **Headline word-cycling animation** — purely aesthetic, not migrated.
  - **Password-manager DOM observer** — the `pm_*` script that strips inline styles from autofilled inputs. Niche workaround, easy to add back if needed.
- **Vue 3 / AD-Vue 4 syntax changes applied:**
  - `<template slot="X">` → `<template #X>` (Vue 3 slot syntax)
  - `<a-icon slot="prefix" type="user">` → `<template #prefix><UserOutlined /></template>` with explicit imports from `@ant-design/icons-vue`. **AD-Vue 4 removed the generic `<a-icon>` — every icon must be imported individually.**
  - `v-model.trim` on `a-input` → `v-model:value` (AD-Vue 4 uses named v-model on inputs). The `.trim` modifier is dropped; trim manually if needed.
  - `new Vue({ el: '#app', delimiters, data, methods })` → `createApp(LoginPage).use(Antd).mount('#app')` with `<script setup>` and Composition API.
  - `mounted()` → `onMounted()`.
  - `el: '#app', delimiters: ['[[', ']]']` is gone — SFCs use the standard `{{ }}`.

### Phase 3 notes

- `web/assets/js/util/index.js` (927 lines, 21 classes) → `frontend/src/utils/legacy.js`. All classes prefixed with `export`. Barrel `frontend/src/utils/index.js` re-exports for cleaner consumer imports.
- `Vue.prototype.$message[...]` inside `HttpUtil._handleMsg` was replaced with a direct `import { message } from 'ant-design-vue'`. Vue 3 has no `Vue.prototype`.
- `RandomUtil.randomShadowsocksPassword` previously defaulted to `SSMethods.BLAKE3_AES_256_GCM` from inbound.js, which would create a circular import. Replaced with the literal string default `'2022-blake3-aes-256-gcm'`.
- `MediaQueryMixin` removed from utils. Replaced by `frontend/src/composables/useMediaQuery.js`, a Vue 3 composable returning a reactive `isMobile`.
- `web/assets/js/axios-init.js` was an imperative side-effect script. Wrapped as `setupAxios()` which the app calls once at startup. `Qs` global → `import qs from 'qs'`.
- `web/assets/js/websocket.js` exported as `WebSocketClient`. The bottom `window.wsClient = ...` line was removed; pages instantiate the client themselves with the basePath they need.
- Models (`inbound`, `outbound`, `dbinbound`, `setting`, `reality_targets`) copied verbatim with `export` added and the imports they need from utils/legacy.js wired up.
- `subscription.js` deferred to Phase 5 — it's a Vue 2 mount, not a util.
- Smoke test added to `frontend/src/App.vue` exercising `SizeFormatter`, `RandomUtil`, `Wireguard`, and `useMediaQuery`. If `npm run dev` renders it correctly, Phase 3 is verified.
