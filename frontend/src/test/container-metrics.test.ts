import { describe, expect, it } from 'vitest';

import { Status } from '@/models/status';
import { StatusSchema } from '@/schemas/status';

describe('container metric scope', () => {
  it('preserves backend scope and availability through validation into the UI model', () => {
    const raw = StatusSchema.parse({
      resourceScope: 'container',
      unavailableMetrics: ['cpu'],
      cpu: 0,
      cpuCores: 2,
      logicalPro: 2,
      mem: { current: 134217728, total: 2147483648 },
      swap: { current: 19357696, total: 536870912 },
      uptime: 52624,
      loads: [1.25, 2.5, 3.75],
    });
    const status = new Status(raw);
    expect(status.resourceScope).toBe('container');
    expect(status.logicalPro).toBe(2);
    expect(status.metricAvailable('cpu')).toBe(false);
    expect(status.metricAvailable('swap')).toBe(true);
    expect(status.swap.current).toBe(19357696);
    expect(status.swap.total).toBe(536870912);
    expect(status.swap.percent).toBeCloseTo(3.605688, 1);
    expect(status.loads).toEqual([1.25, 2.5, 3.75]);
    expect(status.metricAvailable('load')).toBe(true);
  });

  it('keeps legacy responses available and distinguishes disabled from missing swap', () => {
    const legacy = new Status({ swap: { current: 0, total: 0 } });
    expect(legacy.resourceScope).toBe('');
    expect(legacy.metricAvailable('swap')).toBe(true);
    expect(legacy.swap.percent).toBe(0);
    const missing = new Status({ unavailableMetrics: ['swap'] });
    expect(missing.metricAvailable('swap')).toBe(false);
  });
});
