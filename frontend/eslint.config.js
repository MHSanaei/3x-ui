import js from '@eslint/js';
import vue from 'eslint-plugin-vue';
import vueParser from 'vue-eslint-parser';
import globals from 'globals';

export default [
  { ignores: ['node_modules/**', '../web/dist/**'] },
  js.configs.recommended,
  ...vue.configs['flat/recommended'],
  {
    files: ['**/*.{js,vue}'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'module',
      parser: vueParser,
      parserOptions: {
        ecmaFeatures: { jsx: false },
      },
      globals: {
        ...globals.browser,
        ...globals.node,
        // Legacy script tags inject a couple of helpers on window before
        // the SPA boots; declared here so no-undef stops flagging them.
        getRandomRealityTarget: 'readonly',
      },
    },
    rules: {
      'no-unused-vars': ['warn', {
        argsIgnorePattern: '^_',
        varsIgnorePattern: '^_',
        caughtErrorsIgnorePattern: '^_',
      }],
      'no-empty': ['error', { allowEmptyCatch: true }],
      'no-case-declarations': 'off',

      // Stylistic rules from vue/recommended that don't match the
      // existing codebase formatting. Disable rather than churn the
      // whole tree to satisfy them.
      'vue/multi-word-component-names': 'off',
      'vue/no-v-html': 'off',
      'vue/html-self-closing': 'off',
      'vue/max-attributes-per-line': 'off',
      'vue/singleline-html-element-content-newline': 'off',
      'vue/multiline-html-element-content-newline': 'off',
      'vue/html-indent': 'off',
      'vue/html-closing-bracket-newline': 'off',
      'vue/attributes-order': 'off',
      'vue/first-attribute-linebreak': 'off',
      'vue/one-component-per-file': 'off',
      'vue/order-in-components': 'off',
      'vue/attribute-hyphenation': 'off',
      'vue/v-on-event-hyphenation': 'off',

      // Pervasive in form components ported from the Vue 2 codebase
      // (parent passes a reactive object; child mutates it in place).
      // Properly fixing this means rewiring those components to emit
      // updates — a meaningful architectural change, separate task.
      'vue/no-mutating-props': 'off',
    },
  },
];
