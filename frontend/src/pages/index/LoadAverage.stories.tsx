import { ConfigProvider } from 'antd';
import { DashboardOutlined } from '@ant-design/icons';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';
import { I18nextProvider, initReactI18next } from 'react-i18next';
import { createInstance } from 'i18next';

import { Status } from '@/models/status';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import faIR from '../../../../internal/web/translation/fa-IR.json';
import enUS from '../../../../internal/web/translation/en-US.json';
import LoadAverage from './LoadAverage';
import VitalTile from './VitalTile';
import './IndexPage.css';

const locales = createInstance();
void locales.use(initReactI18next).init({
  lng: 'en-US',
  fallbackLng: 'en-US',
  resources: { 'en-US': { translation: enUS }, 'fa-IR': { translation: faIR } },
  interpolation: { escapeValue: false, prefix: '{', suffix: '}' },
  returnNull: false,
});

function LoadAveragePreview({
  loads,
  logicalPro = 4,
  resourceScope = '',
  rtl = false,
}: {
  loads: number[];
  logicalPro?: number;
  resourceScope?: string;
  rtl?: boolean;
}) {
  const status = new Status({ loads, logicalPro, resourceScope });
  const { isMobile } = useMediaQuery();
  return (
    <I18nextProvider i18n={locales}>
      <meta name="viewport" content="width=device-width, initial-scale=1" />
      <ConfigProvider direction={rtl ? 'rtl' : 'ltr'}>
        <div className="index-page" dir={rtl ? 'rtl' : 'ltr'} style={{ maxWidth: 360 }}>
          <VitalTile
            icon={<DashboardOutlined aria-hidden />}
            label="CPU"
            percent={72}
            statusColor="#1677ff"
            detail={`4 Cores / ${logicalPro} Threads`}
            footLeft="Avg 52%"
            footRight="Peak 89%"
            data={[35, 52, 40, 62, 89, 72]}
            isMobile={isMobile}
            extraContent={<LoadAverage status={status} />}
          />
        </div>
      </ConfigProvider>
    </I18nextProvider>
  );
}

const meta = {
  title: 'Monitoring/Load average',
  component: LoadAveragePreview,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Raw load averages in the existing CPU tile, with capacity colors only for comparable CPU scope.',
      },
    },
  },
  args: { loads: [1.2, 3.6, 4.8], logicalPro: 4, resourceScope: '', rtl: false },
  argTypes: {
    loads: { description: 'Raw one-, five- and fifteen-minute load measurements.' },
    logicalPro: { description: 'Available logical CPUs for normalization.' },
    resourceScope: { description: 'Non-empty scope metadata disables host-load normalization.' },
    rtl: { description: 'Preview right-to-left layout.' },
  },
} satisfies Meta<typeof LoadAveragePreview>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Mixed: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByLabelText('1m: 1.20 — Healthy')).toBeVisible();
    await expect(canvas.getByLabelText('5m: 3.60 — Near capacity')).toBeVisible();
    await expect(canvas.getByLabelText('15m: 4.80 — Above capacity')).toBeVisible();
    await expect(canvas.getByText('72.0')).toBeVisible();
    await expect(canvas.getByText('Avg 52%')).toBeVisible();
    await expect(canvas.getByRole('img', { name: 'CPU: 72%' })).toBeVisible();
  },
};
export const Healthy: Story = { args: { loads: [0, 1, 2] } };
export const NearCapacity: Story = { args: { loads: [4, 3.99, 3.6] } };
export const Overloaded: Story = { args: { loads: [4.01, 6, 8] } };
export const Unavailable: Story = {
  args: { loads: [] },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByLabelText('1m: — — Unavailable')).toBeVisible();
  },
};
export const Container: Story = { args: { resourceScope: 'container' } };
export const PersianRTL: Story = {
  args: { rtl: true },
  beforeEach: async () => {
    await locales.changeLanguage('fa-IR');
    return async () => {
      await locales.changeLanguage('en-US');
    };
  },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('button', { name: 'میانگین بار' })).toBeVisible();
  },
};
