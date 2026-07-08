import { describe, it, expect } from 'vitest';
import { normalizeBase, joinUrl, buildCurl, buildFetchSnippet } from './api-client';

describe('normalizeBase', () => {
  it('strips a trailing slash', () => {
    expect(normalizeBase('https://panel.example.com:2053/')).toBe('https://panel.example.com:2053');
  });

  it('leaves a clean base unchanged', () => {
    expect(normalizeBase('https://panel.example.com:2053')).toBe('https://panel.example.com:2053');
  });
});

describe('joinUrl', () => {
  it('joins with exactly one slash regardless of input slashes', () => {
    expect(joinUrl('https://x.com/', 'panel/api/inbounds/list')).toBe(
      'https://x.com/panel/api/inbounds/list',
    );
    expect(joinUrl('https://x.com', '/panel/api/inbounds/list')).toBe(
      'https://x.com/panel/api/inbounds/list',
    );
  });
});

const base = {
  baseUrl: 'https://panel.example.com:2053',
  token: 'TKN',
  path: '/panel/api/inbounds/list',
};

describe('buildCurl', () => {
  it('GET emits the Bearer header, a single-quoted URL, and no body flag', () => {
    const cmd = buildCurl({ ...base, method: 'GET' });
    expect(cmd).toContain("-X GET");
    expect(cmd).toContain("-H 'Authorization: Bearer TKN'");
    expect(cmd).toContain("'https://panel.example.com:2053/panel/api/inbounds/list'");
    expect(cmd).not.toContain('--data');
    expect(cmd).not.toContain('-d ');
  });

  it('POST with a body emits --data and a JSON content type', () => {
    const cmd = buildCurl({ ...base, method: 'POST', path: '/panel/api/inbounds/add', body: '{"up":0}' });
    expect(cmd).toContain('-X POST');
    expect(cmd).toContain("--data '{\"up\":0}'");
    expect(cmd).toContain("Content-Type: application/json");
  });

  it('POST without a body omits --data', () => {
    const cmd = buildCurl({ ...base, method: 'POST', path: '/panel/api/inbounds/resetAllTraffics' });
    expect(cmd).not.toContain('--data');
  });
});

describe('buildFetchSnippet', () => {
  it('GET sets method + Authorization and no body', () => {
    const snip = buildFetchSnippet({ ...base, method: 'GET' });
    expect(snip).toContain("method: 'GET'");
    expect(snip).toContain("'Authorization': 'Bearer TKN'");
    expect(snip).not.toContain('body:');
  });

  it('POST with a body includes a JSON.stringify body', () => {
    const snip = buildFetchSnippet({ ...base, method: 'POST', path: '/panel/api/inbounds/add', body: '{"up":0}' });
    expect(snip).toContain("method: 'POST'");
    expect(snip).toContain('body: JSON.stringify(');
  });
});
