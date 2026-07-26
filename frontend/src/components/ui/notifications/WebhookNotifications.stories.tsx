import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';

import { AllSetting } from '@/models/setting';
import { WebhookNotifications } from './WebhookNotifications';

const meta = {
    title: 'UI/Notifications/WebhookNotifications',
    component: WebhookNotifications,
    tags: ['autodocs'],
    parameters: {
        layout: 'padded',
        docs: {
            description: {
                component:
                    'Grid of grouped event checkboxes on the settings page that picks which panel events (outbound/node health, Xray crashes, CPU/RAM thresholds, login attempts) trigger an HTTP webhook POST, stored as a comma-separated list in webhookEnabledEvents.',
            },
        },
    },
    argTypes: {
        allSetting: {
            description:
                'Panel settings snapshot; webhookEnabledEvents holds the selected event keys and webhookCpu/webhookMemory the alert threshold percentages.',
        },
        updateSetting: {
            description: 'Receives a partial settings patch when an event is toggled or a threshold input changes.',
        },
    },
} satisfies Meta<typeof WebhookNotifications>;

export default meta;

type Story = StoryObj<typeof meta>;

function StatefulDemo({ initial }: { initial: AllSetting }) {
    const [settings, setSettings] = useState(initial);
    return (
        <WebhookNotifications
            allSetting={settings}
            updateSetting={(patch) => setSettings((prev) => new AllSetting({ ...prev, ...patch }))}
        />
    );
}

const placeholderArgs = {
    allSetting: new AllSetting(),
    updateSetting: () => undefined,
};

export const NothingSelected: Story = {
    args: placeholderArgs,
    render: () => <StatefulDemo initial={new AllSetting()} />,
};

export const SystemThresholdAlerts: Story = {
    args: placeholderArgs,
    render: () => (
        <StatefulDemo
            initial={new AllSetting({ webhookEnabledEvents: 'cpu.high,memory.high', webhookCpu: 85, webhookMemory: 90 })}
        />
    ),
};

export const InfrastructureOnly: Story = {
    args: placeholderArgs,
    render: () => (
        <StatefulDemo initial={new AllSetting({ webhookEnabledEvents: 'outbound.down,node.down,node.up,xray.crash' })} />
    ),
};

export const AllEventsEnabled: Story = {
    args: placeholderArgs,
    render: () => (
        <StatefulDemo
            initial={
                new AllSetting({
                    webhookEnabledEvents:
                        'outbound.down,outbound.up,xray.crash,node.down,node.up,cpu.high,memory.high,login.attempt',
                    webhookCpu: 80,
                    webhookMemory: 80,
                })
            }
        />
    ),
};