import coreWebVitals from 'eslint-config-next/core-web-vitals';
import typescript from 'eslint-config-next/typescript';

/** @type {import('eslint').Linter.Config[]} */
const config = [
  {
    ignores: [
      '.next/**',
      '.source/**',
      'out/**',
      'node_modules/**',
      'next-env.d.ts',
      // Generated API reference pages (fumadocs-openapi output)
      'content/docs/**/reference/api/**',
    ],
  },
  ...coreWebVitals,
  ...typescript,
];

export default config;
