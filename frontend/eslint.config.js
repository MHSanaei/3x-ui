import js from '@eslint/js';
import tseslint from 'typescript-eslint';
import reactHooks from 'eslint-plugin-react-hooks';
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
];
