import { useMemo, useState } from 'react';
import { message } from 'antd';
import { CheckOutlined, CopyOutlined } from '@ant-design/icons';
import { ClipboardManager } from '@/utils';
import './CodeBlock.css';

interface CodeBlockProps {
  code?: string;
  lang?: string;
}

function escapeHtml(str: string): string {
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function highlightJson(str: string): string {
  const escaped = escapeHtml(str);
  return escaped.replace(
    /("(?:[^"\\]|\\.)*")\s*(:)|("(?:[^"\\]|\\.)*")|(-?\d+\.?\d*(?:[eE][+-]?\d+)?)\b|(true|false)|(null)|([{}[\]])/g,
    (_m, key, colon, string, number, bool, nil) => {
      if (colon) return `<span class="json-key">${key}</span>${colon}`;
      if (string) return `<span class="json-string">${string}</span>`;
      if (number) return `<span class="json-number">${number}</span>`;
      if (bool) return `<span class="json-boolean">${bool}</span>`;
      if (nil) return `<span class="json-null">${nil}</span>`;
      return _m;
    },
  );
}

export default function CodeBlock({ code = '', lang = 'json' }: CodeBlockProps) {
  const [copied, setCopied] = useState(false);
  const [messageApi, messageContextHolder] = message.useMessage();

  const highlighted = useMemo(
    () => (lang === 'json' ? highlightJson(code) : escapeHtml(code)),
    [code, lang],
  );

  async function copyCode() {
    const ok = await ClipboardManager.copyText(code);
    if (ok) {
      setCopied(true);
      messageApi.success('Copied');
      window.setTimeout(() => setCopied(false), 2000);
    } else {
      messageApi.error('Copy failed');
    }
  }

  return (
    <div className="code-block-wrapper">
      {messageContextHolder}
      <div className="code-toolbar">
        <span className="lang-badge">{lang.toUpperCase()}</span>
        <button
          className={`copy-btn${copied ? ' copied' : ''}`}
          onClick={copyCode}
          title={copied ? 'Copied' : 'Copy'}
        >
          {copied ? <CheckOutlined /> : <CopyOutlined />}
        </button>
      </div>
      <pre className={`code-block lang-${lang}`}>
        <code dangerouslySetInnerHTML={{ __html: highlighted }} />
      </pre>
    </div>
  );
}
