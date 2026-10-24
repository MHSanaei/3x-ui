import type { StorybookConfig } from '@storybook/react-vite';

const config: StorybookConfig = {
  framework: {
    name: '@storybook/react-vite',
    options: {},
  },
  stories: ['../src/**/*.stories.@(ts|tsx)'],
  addons: [
    '@storybook/addon-docs',
    '@storybook/addon-a11y',
    '@storybook/addon-vitest'
  ],
  viteFinal: (viteConfig) => {
    if (viteConfig.build) {
      viteConfig.build.outDir = undefined;
      viteConfig.build.emptyOutDir = false;
      // Storybook bundles the whole UI plus the axe engine; hold it to the app's chunk ceiling.
      viteConfig.build.chunkSizeWarningLimit = 1500;
      if (viteConfig.build.rollupOptions) {
        viteConfig.build.rollupOptions.input = undefined;
      }
    }
    if (viteConfig.experimental) {
      viteConfig.experimental.renderBuiltUrl = undefined;
    }
    return viteConfig;
  },
};

export default config;
