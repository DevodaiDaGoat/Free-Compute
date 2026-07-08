'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { apiFetch, getGatewayUrl, getTokens } from '../../boot/BootSequence';

export default function TerminalApp() {
  const [input, setInput] = useState('');
  const [history, setHistory] = useState<string[]>([
    'FreeCompute WebOS Terminal v0.1.0',
    'Type "help" for available commands.',
    '',
  ]);
  const [currentPath, setCurrentPath] = useState('~');
  const [env, setEnv] = useState<Record<string, string>>({});
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
    const handler = () => inputRef.current?.focus();
    window.addEventListener('click', handler);
    return () => window.removeEventListener('click', handler);
  }, []);

  const executeCommand = useCallback(async (cmd: string) => {
    const tokens = getTokens();
    const parts = cmd.trim().split(/\s+/);
    const command = parts[0].toLowerCase();
    const args = parts.slice(1);

    switch (command) {
      case 'help':
        return [
          'Available commands:',
          '  help              - Show this help',
          '  echo <text>       - Print text',
          '  clear             - Clear terminal',
          '  date              - Show current date/time',
          '  whoami            - Show current user',
          '  tailscale         - Show Tailscale status',
          '  webrtc            - Test WebRTC connection',
          '  gateway           - Show gateway status',
          '  proxy <url>       - Test proxy connection',
          '  ssh <host>        - SSH tunnel (via WebSocket)',
          '  ls                - List files (cloud drive)',
          '  cat <file>        - Read file from drive',
          '  storage           - Show storage quota',
          '  devices           - List active input devices',
          '  session           - Show active sessions',
          '  env               - Show environment variables',
          '  export KEY=VALUE  - Set environment variable',
        ].join('\n');

      case 'clear':
        setHistory([]);
        return '';

      case 'date':
        return new Date().toString();

      case 'whoami':
        if (tokens) return `Authenticated user`;
        return 'Not logged in. Use WebOS login screen.';

      case 'echo':
        return args.join(' ');

      case 'gateway':
        try {
          const health = await fetch(`${getGatewayUrl()}/healthz`).then(r => r.json());
          const caps = await fetch(`${getGatewayUrl()}/capabilities`).then(r => r.json());
          return `Gateway: ${getGatewayUrl()}\nStatus: ${JSON.stringify(health)}\nProtocols: ${caps.protocols?.join(', ') || 'N/A'}\nModes: ${caps.routeModes?.join(', ') || 'N/A'}`;
        } catch (e: any) {
          return `Error: ${e.message}`;
        }

      case 'webrtc':
        return 'WebRTC: Use POST /webrtc/ with {"clientId":"...","preset":"fast"}';

      case 'tailscale':
        try {
          const hosts = await fetch(`${getGatewayUrl()}/tailscale/hosts`).then(r => r.json());
          return `Tailscale Hosts:\n${(hosts.hosts || []).map((h: any) => `  ${h.tailscaleIp} ${h.hostName} (${h.vms?.length || 0} VMs)`).join('\n') || '  None registered'}`;
        } catch (e: any) {
          return `Error: ${e.message}`;
        }

      case 'proxy':
        if (args.length === 0) return 'Usage: proxy <target-url>';
        try {
          const testRes = await fetch(`${getGatewayUrl()}/proxy/web/${args[0]}`, {
            headers: tokens ? { Authorization: `Bearer ${tokens.accessToken}` } : {},
          });
          return `Proxy test: ${testRes.status} ${testRes.statusText}`;
        } catch (e: any) {
          return `Error: ${e.message}`;
        }

      case 'ssh':
        if (args.length === 0) return 'Usage: ssh <host:port>\nConnect via WebSocket tunnel: ws://gateway/ws/{routeID}';
        return `SSH tunnel to ${args[0]}: Use WebSocket at /ws/{routeID} or CONNECT at /connect/{routeID}`;

      case 'ls':
        try {
          if (!tokens) return 'Not authenticated';
          const params = new URLSearchParams({ userId: tokens.accessToken, path: '' });
          const res = await fetch(`${getGatewayUrl()}/storage/list?${params}`);
          const data = await res.json();
          const files = data.files || [];
          if (files.length === 0) return 'No files';
          return files.map((f: any) => `${f.isDir ? 'd' : '-'} ${f.name} (${formatSize(f.size)})`).join('\n');
        } catch (e: any) {
          return `Error: ${e.message}`;
        }

      case 'cat':
        if (args.length === 0) return 'Usage: cat <path>';
        try {
          if (!tokens) return 'Not authenticated';
          const params = new URLSearchParams({ userId: tokens.accessToken, path: args[0] });
          const res = await fetch(`${getGatewayUrl()}/storage/download?${params}`);
          if (!res.ok) return 'File not found';
          return await res.text();
        } catch (e: any) {
          return `Error: ${e.message}`;
        }

      case 'storage':
        try {
          if (!tokens) return 'Not authenticated';
          const profile = await apiFetch('/auth/profile');
          const used = profile.storageUsed || 0;
          const quota = profile.storageQuota || 107374182400;
          return `Storage: ${formatSize(used)} / ${formatSize(quota)} (${Math.round((used/quota)*100)}%)`;
        } catch (e: any) {
          return `Error: ${e.message}`;
        }

      case 'devices':
        return 'Input devices: keyboard, mouse, touch, gamepad';

      case 'session':
        return 'Active sessions: Check GET /sessions/';

      case 'env':
        return Object.entries(env).map(([k, v]) => `${k}=${v}`).join('\n') || '(empty)';

      case 'export':
        if (args.length === 0) return 'Usage: export KEY=VALUE';
        const eq = args[0].indexOf('=');
        if (eq === -1) return 'Format: KEY=VALUE';
        const key = args[0].slice(0, eq);
        const val = args[0].slice(eq + 1);
        setEnv(prev => ({ ...prev, [key]: val }));
        return '';

      default:
        return `Unknown command: ${command}. Type "help" for commands.`;
    }
  }, [env]);

  const handleSubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = input.trim();
    if (!trimmed) return;
    setHistory(prev => [...prev, `$ ${trimmed}`]);
    try {
      const output = await executeCommand(trimmed);
      if (output) {
        setHistory(prev => [...prev, output]);
      }
    } catch (err: any) {
      setHistory(prev => [...prev, `Error: ${err.message}`]);
    }
    setHistory(prev => [...prev, '']);
    setInput('');
  }, [input, executeCommand]);

  return (
    <div
      style={{ background: '#0a0e14', color: '#b3b1ad', height: '100%', display: 'flex', flexDirection: 'column', fontFamily: 'monospace', fontSize: 13, cursor: 'text' }}
      onClick={() => inputRef.current?.focus()}
    >
      <div style={{ flex: 1, overflow: 'auto', padding: '8px 12px' }}>
        {history.map((line, i) => (
          <div key={i} style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all', lineHeight: 1.5 }}>
            {line.startsWith('$ ') ? <span><span style={{ color: '#b8cc52' }}>$</span> {line.slice(2)}</span> : line}
          </div>
        ))}
      </div>
      <form onSubmit={handleSubmit} style={{ display: 'flex', padding: '4px 12px', borderTop: '1px solid #1a1e24' }}>
        <span style={{ color: '#b8cc52', marginRight: 8 }}>$</span>
        <input
          ref={inputRef}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          style={{ background: 'none', border: 'none', color: '#b3b1ad', flex: 1, outline: 'none', fontFamily: 'monospace', fontSize: 13 }}
          autoFocus
        />
      </form>
    </div>
  );
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}
