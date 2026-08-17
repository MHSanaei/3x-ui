import type { Meta, StoryObj } from '@storybook/react-vite';

import Sparkline from './Sparkline';

const wave = Array.from({ length: 48 }, (_, i) => 45 + Math.round(28 * Math.sin(i / 4) + (i % 5) * 3));
const inverse = wave.map((v) => Math.max(0, 100 - v));

const meta = {
  title: 'Viz/Sparkline',
  component: Sparkline,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Compact canvas line chart (uPlot) for CPU, memory, and traffic trends. Supports up to three series, optional axes/grid, a hover tooltip, min/max markers, reference lines, and light/dark theming.',
      },
    },
  },
  argTypes: {
    data: { description: 'Primary series values, oldest to newest.' },
    data2: { description: 'Optional second series (e.g. download vs upload).' },
    data3: { description: 'Optional third series.' },
    height: { description: 'Chart height in pixels.' },
    name1: { description: 'Legend/tooltip label for the primary series.' },
    name2: { description: 'Legend/tooltip label for the second series.' },
    showAxes: { description: 'Render x/y axes and tick labels.' },
    showGrid: { description: 'Draw horizontal grid lines.' },
    showTooltip: { description: 'Show a value tooltip on hover.' },
    extrema: { description: 'Highlight the min and max points (single-series only).' },
  },
} satisfies Meta<typeof Sparkline>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: { data: wave, height: 80 },
};

export const AxesAndGrid: Story = {
  args: { data: wave, height: 140, showAxes: true, showGrid: true, name1: 'CPU' },
};

export const Extrema: Story = {
  args: { data: wave, height: 140, name1: 'CPU', extrema: { show: true } },
};

export const MultiSeriesTooltip: Story = {
  args: {
    data: wave,
    data2: inverse,
    name1: 'Upload',
    name2: 'Download',
    height: 140,
    showTooltip: true,
    showAxes: true,
  },
};
