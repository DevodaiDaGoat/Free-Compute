'use client';

import { useState, useCallback, useRef } from 'react';
import { ArrowLeft, ArrowRight, RefreshCw, Home, Globe } from 'lucide-react';

const PROXY_BASE = '/api/browser/proxy?url=';
const HOME_URL = 'https://example.com';

function ensureScheme(u: string): string {
  const t = u.trim();
  if (!t) return '';
  if (/^[a-zA-Z][a-zA-Z\d+.-]*:\/\//.test(t)) return t;
  return `https://${t}`;
}

function proxyFetchUrl(target: string): string {
  return `${PROXY_BASE}${encodeURIComponent(target)}`;
}

function baseHrefFor(target: string): string {
  return `${PROXY_BASE}${encodeURIComponent(target)}/`;
}

function injectBase(html: string, base: string): string {
  const tag = `<base href="${base}">`;
  const headOpen = /<head[^>]*>/i.exec(html);
  if (headOpen) {
    const idx = headOpen.index + headOpen[0].length;
    return html.slice(0, idx) + tag + html.slice(idx);
  }
  const htmlOpen = /<html[^>]*>/i.exec(html);
  if (htmlOpen) {
    const idx = htmlOpen.index + htmlOpen[0].length;
    return html.slice(0, idx) + `<head>${tag}</head>` + html.slice(idx);
  }
  return `<head>${tag}</head>` + html;
}

function extractTitle(html: string): string {
  const m = /<title[^>]*>([\s\S]*?)<\/title>/i.exec(html);
  return m ? m[1].trim() : 'Untitled';
}

type Status = 'idle' | 'loading' | 'error' | 'ok';

export default function BrowserApp() {
  const [input, setInput] = useState(HOME_URL);
  const [currentUrl, setCurrentUrl] = useState(HOME_URL);
  const [history, setHistory] = useState<string[]>([HOME_URL]);
  const [index, setIndex] = useState(0);
  const [status, setStatus] = useState<Status>('idle');
  const [errorMsg, setErrorMsg] = useState('');
  const [srcDoc, setSrcDoc] = useState(
    '<h1>FreeCompute Browser</h1><p>Enter a URL and press Go to navigate.</p>'
  );
  const [title, setTitle] = useState('FreeCompute Browser');
  const abortRef = useRef<AbortController | null>(null);

  const navigateTo = useCallback(
    (raw: string, { push = true }: { push?: boolean } = {}) => {
      const target = ensureScheme(raw);
      if (!target) return;

      abortRef.current?.abort();
      const ac = new AbortController();
      abortRef.current = ac;

      setStatus('loading');
      setErrorMsg('');
      setCurrentUrl(target);
      setInput(target);

      const fetchUrl = proxyFetchUrl(target);
      fetch(fetchUrl, { signal: ac.signal, redirect: 'follow' })
        .then(async (res) => {
          const ct = res.headers.get('content-type') || '';
          const text = await res.text();
          if (!res.ok) {
            setStatus('error');
            setErrorMsg(`HTTP ${res.status} ${res.statusText}`);
            setSrcDoc(
              `<div style="font-family:monospace;padding:16px;color:#f85149">HTTP ${res.status} ${res.statusText} while loading ${target}</div>`
            );
            return;
          }
          if (ct.includes('text/html') || /^<!doctype html/i.test(text) || /<html/i.test(text)) {
            const withBase = injectBase(text, baseHrefFor(target));
            setTitle(extractTitle(text) || target);
            setSrcDoc(withBase);
            setStatus('ok');
          } else {
            setTitle(target);
            setSrcDoc(
              `<pre style="margin:0;padding:16px;background:#0d1117;color:#c9d1d9;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:13px;white-space:pre-wrap;word-break:break-word;border:1px solid #30363d">${escapeHtml(
                text
              )}</pre>`
            );
            setStatus('ok');
          }
          if (push) {
            setHistory((h) => {
              const next = h.slice(0, index + 1);
              next.push(target);
              return next;
            });
            setIndex((i) => i + 1);
          }
        })
        .catch((err) => {
          if (ac.signal.aborted) return;
          setStatus('error');
          setErrorMsg(err?.message || 'Network error');
          setSrcDoc(
            `<div style="font-family:monospace;padding:16px;color:#f85149">Could not load ${target}.<br/>${escapeHtml(
              String(err?.message || err)
            )}</div>`
          );
        });
    },
    [index]
  );

  const goBack = () => {
    if (index > 0) {
      const ni = index - 1;
      setIndex(ni);
      navigateTo(history[ni], { push: false });
    }
  };
  const goForward = () => {
    if (index < history.length - 1) {
      const ni = index + 1;
      setIndex(ni);
      navigateTo(history[ni], { push: false });
    }
  };
  const refresh = () => navigateTo(currentUrl);
  const goHome = () => navigateTo(HOME_URL);

  const btn = (disabled: boolean): React.CSSProperties => ({
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 30,
    height: 30,
    background: '#21262d',
    color: disabled ? '#484f58' : '#c9d1d9',
    border: '1px solid #30363d',
    borderRadius: 6,
    cursor: disabled ? 'default' : 'pointer',
    fontSize: 14,
  });

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        background: '#0d1117',
        color: '#c9d1d9',
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          padding: '6px 8px',
          background: '#161b22',
          borderBottom: '1px solid #30363d',
        }}
      >
        <button style={btn(index <= 0)} onClick={goBack} disabled={index <= 0} title="Back">
          <ArrowLeft size={16} />
        </button>
        <button
          style={btn(index >= history.length - 1)}
          onClick={goForward}
          disabled={index >= history.length - 1}
          title="Forward"
        >
          <ArrowRight size={16} />
        </button>
        <button style={btn(false)} onClick={refresh} title="Refresh">
          <RefreshCw size={16} />
        </button>
        <button style={btn(false)} onClick={goHome} title="Home">
          <Home size={16} />
        </button>
        <div
          style={{
            display: 'flex',
            flex: 1,
            alignItems: 'center',
            gap: 6,
            padding: '5px 10px',
            background: '#0d1117',
            border: '1px solid #30363d',
            borderRadius: 6,
            color: '#ccc',
            fontSize: 12,
          }}
        >
          <Globe size={14} color="#8b949e" />
          <input
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && navigateTo(input)}
            style={{
              flex: 1,
              background: 'transparent',
              border: 'none',
              outline: 'none',
              color: '#c9d1d9',
              fontSize: 12,
            }}
            placeholder="Enter a URL..."
          />
        </div>
      </div>

      <iframe
        srcDoc={srcDoc}
        style={{ flex: 1, border: 'none', background: '#fff' }}
        title="browser-content"
        // Do NOT combine allow-scripts with allow-same-origin — the MDN
        // sandbox docs are explicit that this makes the iframe effectively
        // un-sandboxed (the child can strip its own sandbox attribute and
        // reach parent origin cookies/localStorage). Dropping
        // allow-same-origin keeps scripting for proxied pages but pins the
        // iframe to an opaque origin, so it cannot exfiltrate our JWT.
        sandbox="allow-scripts allow-forms allow-popups allow-modals allow-pointer-lock"
      />

      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '3px 10px',
          background: '#161b22',
          borderTop: '1px solid #30363d',
          color: '#8b949e',
          fontSize: 11,
          fontFamily: 'ui-monospace,SFMono-Regular,Menlo,monospace',
        }}
      >
        <span
          style={{
            width: 8,
            height: 8,
            borderRadius: '50%',
            background:
              status === 'loading'
                ? '#d29922'
                : status === 'error'
                ? '#f85149'
                : '#238636',
            flexShrink: 0,
          }}
        />
        <span style={{ flexShrink: 0 }}>
          {status === 'loading'
            ? 'Loading…'
            : status === 'error'
            ? `Error${errorMsg ? `: ${errorMsg}` : ''}`
            : 'Ready'}
        </span>
        <span
          style={{
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
            flex: 1,
          }}
          title={title}
        >
          {title}
        </span>
        <span style={{ flexShrink: 0 }}>{currentUrl}</span>
      </div>
    </div>
  );
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}
