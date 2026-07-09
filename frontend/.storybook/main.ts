import type { StorybookConfig } from '@storybook/react-vite';

const config: StorybookConfig = {
  framework: {
    name: '@storybook/react-vite',
    options: {},
  },
  stories: ['../src/**/*.stories.@(ts|tsx)'],
  addons: ['@storybook/addon-docs', '@storybook/addon-a11y'],
  viteFinal: (viteConfig) => {
    if (viteConfig.build) {
      viteConfig.build.outDir = undefined;
      viteConfig.build.emptyOutDir = false;
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
