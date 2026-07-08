'use client';

import { useState } from 'react';
import {
  buildScriptCommand,
  buildDockerRun,
  buildDockerCompose,
  type InstallMethod,
  type InstallOptions,
} from '@/lib/xray/install';
import { ToolFrame } from './tool-frame';
import { TextField, SelectField, CheckboxField } from './shared/fields';
import { OutputBlock } from './shared/output-block';

export function InstallCommandBuilder() {
  const [method, setMethod] = useState<InstallMethod>('script');
  const [version, setVersion] = useState('');
  const [enableFail2ban, setEnableFail2ban] = useState(true);
  const [panelPort, setPanelPort] = useState('');
  const [webBasePath, setWebBasePath] = useState('');

  const options: InstallOptions = {
    method,
    version,
    enableFail2ban,
    panelPort,
    webBasePath,
  };

  return (
    <ToolFrame
      title="Install command builder"
      description="Build the exact install command for your setup. It is assembled in your browser."
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <SelectField
          label="Method"
          value={method}
          onChange={(v) => setMethod(v as InstallMethod)}
          options={['script', 'docker']}
        />
        <TextField
          label="Version"
          value={version}
          onChange={setVersion}
          placeholder="latest"
          hint="blank = latest stable · a tag like v3.4.0 · or dev-latest for the rolling dev build"
        />
        {method === 'docker' ? (
          <>
            <TextField
              label="Panel port"
              value={panelPort}
              onChange={setPanelPort}
              placeholder="2053"
            />
            <TextField
              label="Web base path"
              value={webBasePath}
              onChange={setWebBasePath}
              placeholder="/panel"
            />
          </>
        ) : null}
      </div>

      <div className="mt-3">
        <CheckboxField
          label="Enable Fail2ban"
          checked={enableFail2ban}
          onChange={setEnableFail2ban}
        />
      </div>

      <div className="mt-4 grid grid-cols-1 gap-4">
        {method === 'script' ? (
          <OutputBlock label="Run on your server" value={buildScriptCommand(options)} />
        ) : (
          <>
            <OutputBlock label="docker run" value={buildDockerRun(options)} />
            <OutputBlock label="docker-compose.yml" value={buildDockerCompose(options)} />
          </>
        )}
      </div>
    </ToolFrame>
  );
}
