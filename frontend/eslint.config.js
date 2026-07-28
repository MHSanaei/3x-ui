import js from '@eslint/js';
import tseslint from 'typescript-eslint';
import reactHooks from 'eslint-plugin-react-hooks';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import globals from 'globals';

export default [
  { ignores: ['node_modules/**', '../internal/web/dist/**'] },
  js.configs.recommended,
  ...tseslint.configs.recommended.map((config) => ({
    ...config,
    files: ['**/*.{ts,tsx}'],
  })),
  {
    files: ['**/*.{ts,tsx}'],
    plugins: {
      'react-hooks': reactHooks,
    },
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'module',
      globals: {
        ...globals.browser,
      },
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      '@typescript-eslint/no-unused-vars': ['warn', {
        argsIgnorePattern: '^_',
        varsIgnorePattern: '^_',
        caughtErrorsIgnorePattern: '^_',
      }],
      // Zod migration goal (Step 7): every production module is held to
      // strict no-explicit-any. The two legacy class files at the bottom
      // of the rule list keep their existing file-level eslint-disable
      // until DBInbound is migrated off Inbound.toInbound() — see the
      // migration spec Non-Goals section.
      '@typescript-eslint/no-explicit-any': 'error',
      'no-empty': ['error', { allowEmptyCatch: true }],
      'react-hooks/set-state-in-effect': 'off',
      'react-hooks/purity': 'off',
      'react-hooks/react-compiler': 'off',
      'react-hooks/preserve-manual-memoization': 'off',
      'react-hooks/immutability': 'off',
      'react-hooks/refs': 'off',
    },
  },
  {
    files: ['**/*.tsx'],
    plugins: { 'jsx-a11y': jsxA11y },
    rules: {
      ...jsxA11y.flatConfigs.recommended.rules,
      'jsx-a11y/no-autofocus': 'off',
    },
  },
  {
    // The settings and xray pages write numeric InputNumber changes straight
    // into state, so a null-collapsing handler (`Number(v) || N`, or the
    // ternary `typeof v === 'number' ? v : N`) turns a cleared field into a
    // stored N — the cleared-port bug, #6121. Handlers here go through
    // onNumber() (src/utils/onNumber.ts) instead. Known limit: a handler
    // extracted into a variable and passed as onChange={handler} is not
    // matched; the inline shapes below are the ones that drift in practice.
    files: ['src/pages/settings/**/*.tsx', 'src/pages/xray/**/*.tsx'],
    rules: {
      'no-restricted-syntax': ['error', {
        selector: 'JSXElement[openingElement.name.name="InputNumber"] JSXAttribute[name.name="onChange"] LogicalExpression[operator="||"] > CallExpression[callee.name="Number"]',
        message: 'A cleared InputNumber must not write a synthetic value; wrap the handler with onNumber() from @/utils/onNumber (see #6127).',
      }, {
        selector: 'JSXElement[openingElement.name.name="InputNumber"] JSXAttribute[name.name="onChange"] ConditionalExpression[test.left.operator="typeof"][alternate.type="Literal"]',
        message: 'A cleared InputNumber must not write a synthetic value; wrap the handler with onNumber() from @/utils/onNumber (see #6127).',
      }, {
        selector: 'JSXElement[openingElement.name.name="InputNumber"] JSXAttribute[name.name="onChange"] LogicalExpression[operator="??"][right.type="Literal"]',
        message: 'A cleared InputNumber must not write a synthetic value; wrap the handler with onNumber() from @/utils/onNumber (see #6127).',
      }],
    },
  },
  {
    // The xray form modals (OutboundFormModal, BalancerFormModal,
    // DnsServerModal, WarpModal, …) stage values behind Zod validation like
    // the clients/inbounds modals do, and some of their fields carry a
    // deliberate clear-means-zero semantic — the direct-write rule above
    // does not apply to them.
    files: ['src/pages/xray/**/*Modal.tsx'],
    rules: {
      'no-restricted-syntax': 'off',
    },
  },
];
