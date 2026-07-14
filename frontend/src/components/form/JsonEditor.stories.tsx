import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { Typography } from 'antd';

import { ThemeProvider } from '@/hooks/useTheme';

import JsonEditor from './JsonEditor';

const warpOutbound = JSON.stringify(
  {
    tag: 'warp-out',
    protocol: 'wireguard',
    settings: {
      secretKey: 'yFXfmXX3Zn5tnpNJ7HAcbLvqcMVioqPDGV1GXn2FeV0=',
      address: ['172.16.0.2/32', '2606:4700:110:8f81::2/128'],
      peers: [
        {
          publicKey: 'bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=',
          allowedIPs: ['0.0.0.0/0', '::/0'],
          endpoint: 'engage.cloudflareclient.com:2408',
        },
      ],
      mtu: 1280,
    },
  },
  null,
  2,
);

const realityStreamSettings = JSON.stringify(
  {
    network: 'tcp',
    security: 'reality',
    realitySettings: {
      show: false,
      dest: 'yahoo.com:443',
      xver: 0,
      serverNames: ['yahoo.com', 'www.yahoo.com'],
      privateKey: 'wLc4dpQvRt8mK1nS9jH2fXaU7yEoB3iZ6vNqTgCkW5A',
      shortIds: ['6ba85179e30d4fc2'],
    },
  },
  null,
  2,
);

const brokenInboundSettings = [
  '{',
  '  "clients": [',
  '    {',
  '      "id": "9f4c3a2b-7d61-4e8a-b5c0-1f2e3d4a5b6c",',
  '      "email": "user1@node-de",',
  '      "flow": "xtls-rprx-vision",',
  '    }',
  '  ],',
  '  "decryption": "none"',
].join('\n');

function ControlledDemo() {
  const [value, setValue] = useState(warpOutbound);
  let parseError = '';
  try {
    JSON.parse(value);
  } catch (err) {
    parseError = err instanceof Error ? err.message : String(err);
  }
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      <JsonEditor value={value} onChange={setValue} minHeight="220px" maxHeight="360px" />
      <Typography.Text type={parseError ? 'danger' : 'success'}>
        {parseError ? `Parse error: ${parseError}` : `Valid JSON (${value.length} chars)`}
      </Typography.Text>
    </div>
  );
}

const meta = {
  title: 'Form/JsonEditor',
  component: JsonEditor,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'CodeMirror-based JSON editor with syntax highlighting, live parse linting, and theme-aware styling. The panel uses it for raw xray config snippets — inbound settings, stream settings, and outbound JSON in the modals and settings pages.',
      },
    },
  },
  decorators: [
    (Story) => (
      <ThemeProvider>
        <Story />
      </ThemeProvider>
    ),
  ],
  argTypes: {
    value: { description: 'JSON document text; the editor resyncs when this prop changes.' },
    onChange: { description: 'Called with the full document text on every edit.' },
    minHeight: { description: 'CSS min-height of the scrollable editor area.' },
    maxHeight: { description: 'CSS max-height before the editor scrolls internally.' },
    readOnly: { description: 'Disables editing while keeping selection and scrolling.' },
  },
} satisfies Meta<typeof JsonEditor>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: { value: warpOutbound },
};

export const ReadOnly: Story = {
  args: { value: realityStreamSettings, readOnly: true, minHeight: '200px' },
};

export const LintErrors: Story = {
  args: { value: brokenInboundSettings, minHeight: '220px' },
};

export const Controlled: Story = {
  args: { value: '' },
  parameters: {
    a11y: {
      config: {
        rules: [{ id: 'scrollable-region-focusable', enabled: false }],
      },
    },
  },
  render: () => <ControlledDemo />,
};
