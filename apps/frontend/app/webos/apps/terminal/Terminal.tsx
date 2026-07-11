'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { apiFetch, getGatewayUrl, getTokens, getUser } from '../../boot/BootSequence';

const C = {
  bg: '#0a0e14',
  text: '#b3b1ad',
  prompt: '#98c379',
  error: '#e06c75',
  info: '#61afef',
};

const FONT = 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace';

const HOME = '/home/user';

type FsNode = { type: 'dir' } | { type: 'file'; content: string };
type Fs = Record<string, FsNode>;

function initialFs(): Fs {
  return {
    '/': { type: 'dir' },
    '/home': { type: 'dir' },
    [HOME]: { type: 'dir' },
    [`${HOME}/readme.txt`]: { type: 'file', content: 'Welcome to FreeCompute WebOS.\nType "help" to list commands.' },
    [`${HOME}/projects`]: { type: 'dir' },
  };
}

function resolvePath(cwd: string, arg: string): string {
  if (!arg || arg === '.') return cwd;
  if (arg === '~') return HOME;
  if (arg.startsWith('~/')) arg = HOME + arg.slice(1);
  const start = arg.startsWith('/') ? arg : `${cwd}/${arg}`;
  const stack: string[] = [];
  for (const p of start.split('/')) {
    if (!p || p === '.') continue;
    if (p === '..') stack.pop();
    else stack.push(p);
  }
  return '/' + stack.join('/');
}

function displayPath(p: string): string {
  if (p === HOME) return '~';
  if (p.startsWith(HOME + '/')) return '~' + p.slice(HOME.length);
  return p;
}

function childrenOf(fs: Fs, dir: string): string[] {
  const prefix = dir === '/' ? '/' : dir + '/';
  return Object.keys(fs)
    .filter((k) => k.startsWith(prefix) && k.slice(prefix.length).indexOf('/') === -1)
    .map((k) => k.slice(prefix.length));
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

export default function TerminalApp() {
  const [input, setInput] = useState('');
  const [lines, setLines] = useState<{ kind: 'in' | 'out' | 'err'; text: string }[]>([
    { kind: 'out', text: 'FreeCompute WebOS Terminal v0.1.0' },
    { kind: 'out', text: 'Type "help" for available commands.' },
  ]);
  const [cwd, setCwd] = useState<string>(HOME);
  const [fs, setFs] = useState<Fs>(initialFs);
  const [cmdHistory, setCmdHistory] = useState<string[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
    const focus = () => inputRef.current?.focus();
    window.addEventListener('click', focus);
    return () => window.removeEventListener('click', focus);
  }, []);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight });
  }, [lines]);

  const username = () => getUser()?.displayName || getTokens()?.accessToken || 'guest';
  const prompt = () => `${username()}@webos:${displayPath(cwd)}$`;

  const gateway = useCallback(
    async (fn: () => Promise<string>) => {
      try {
        return await fn();
      } catch {
        return 'unreachable (demo)';
      }
    },
    []
  );

  const execute = useCallback(
    async (cmd: string): Promise<string> => {
      const tokens = getTokens();
      const parts = cmd.trim().split(/\s+/);
      const command = parts[0].toLowerCase();
      const args = parts.slice(1);

      switch (command) {
        case 'help':
          return [
            'Available commands:',
            '  help              - Show this help',
            '  clear             - Clear the screen',
            '  ls [dir]          - List directory contents',
            '  cd <dir>          - Change directory',
            '  pwd               - Print working directory',
            '  mkdir <name>      - Create a directory',
            '  touch <name>      - Create an empty file',
            '  rm <name>         - Remove a file or directory',
            '  cat <file>        - Print file contents',
            '  echo <text>       - Print text',
            '  whoami            - Show current user',
            '  date              - Show current date/time',
            '  history           - Show command history',
            '  gateway           - Show gateway status',
            '  proxy <url>       - Test proxy connection',
            '  tailscale         - Show Tailscale status',
            '  storage           - Show storage quota',
          ].join('\n');

        case 'clear':
          setLines([]);
          return '';

        case 'echo':
          return args.join(' ');

        case 'date':
          return new Date().toString();

        case 'whoami':
          if (tokens) return `Authenticated as ${username()}`;
          return 'Not logged in. Use the WebOS login screen.';

        case 'pwd':
          return displayPath(cwd);

        case 'ls': {
          const target = args[0] ? resolvePath(cwd, args[0]) : cwd;
          if (!fs[target] || fs[target].type !== 'dir') {
            return `ls: ${args[0] || displayPath(cwd)}: No such directory`;
          }
          const kids = childrenOf(fs, target);
          if (kids.length === 0) return '(empty)';
          return kids
            .map((name) => {
              const node = fs[target === '/' ? `/${name}` : `${target}/${name}`];
              return node.type === 'dir' ? `${name}/` : name;
            })
            .join('\n');
        }

        case 'cd': {
          if (args.length === 0 || args[0] === '~' || args[0] === 'home') {
            setCwd(HOME);
            return '';
          }
          const target = resolvePath(cwd, args[0]);
          if (!fs[target]) return `cd: ${args[0]}: No such file or directory`;
          if (fs[target].type !== 'dir') return `cd: ${args[0]}: Not a directory`;
          setCwd(target);
          return '';
        }

        case 'mkdir': {
          if (args.length === 0) return 'Usage: mkdir <name>';
          const target = resolvePath(cwd, args[0]);
          if (fs[target]) return `mkdir: ${args[0]}: File exists`;
          const parent = target.includes('/') ? target.slice(0, target.lastIndexOf('/')) || '/' : '/';
          if (!fs[parent] || fs[parent].type !== 'dir') return `mkdir: ${args[0]}: No such directory`;
          setFs((prev) => ({ ...prev, [target]: { type: 'dir' } }));
          return '';
        }

        case 'touch': {
          if (args.length === 0) return 'Usage: touch <name>';
          const target = resolvePath(cwd, args[0]);
          if (fs[target] && fs[target].type === 'dir') return `touch: ${args[0]}: Is a directory`;
          if (!fs[target]) {
            const parent = target.includes('/') ? target.slice(0, target.lastIndexOf('/')) || '/' : '/';
            if (!fs[parent] || fs[parent].type !== 'dir') return `touch: ${args[0]}: No such directory`;
            setFs((prev) => ({ ...prev, [target]: { type: 'file', content: '' } }));
          }
          return '';
        }

        case 'rm': {
          if (args.length === 0) return 'Usage: rm <name>';
          const target = resolvePath(cwd, args[0]);
          if (!fs[target]) return `rm: ${args[0]}: No such file or directory`;
          setFs((prev) => {
            const next = { ...prev };
            const prefix = target === '/' ? '/' : target + '/';
            for (const k of Object.keys(next)) {
              if (k === target || k.startsWith(prefix)) delete next[k];
            }
            return next;
          });
          return '';
        }

        case 'cat': {
          if (args.length === 0) return 'Usage: cat <file>';
          const target = resolvePath(cwd, args[0]);
          if (!fs[target]) return `cat: ${args[0]}: No such file or directory`;
          if (fs[target].type === 'dir') return `cat: ${args[0]}: Is a directory`;
          return (fs[target] as { content: string }).content || '';
        }

        case 'history':
          return cmdHistory.map((c, i) => `  ${i + 1}  ${c}`).join('\n') || '(no history)';

        case 'gateway':
          return gateway(async () => {
            const base = getGatewayUrl();
            const health = await fetch(`${base}/healthz`).then((r) => r.json());
            const caps = await fetch(`${base}/capabilities`).then((r) => r.json());
            return `Gateway: ${base}\nStatus: ${JSON.stringify(health)}\nProtocols: ${(caps.protocols || []).join(', ') || 'N/A'}\nModes: ${(caps.routeModes || []).join(', ') || 'N/A'}`;
          });

        case 'proxy':
          if (args.length === 0) return 'Usage: proxy <target-url>';
          return gateway(async () => {
            const base = getGatewayUrl();
            const res = await fetch(`${base}/proxy/web/${args[0]}`, {
              headers: tokens ? { Authorization: `Bearer ${tokens.accessToken}` } : {},
            });
            return `Proxy test: ${res.status} ${res.statusText}`;
          });

        case 'tailscale':
          return gateway(async () => {
            const hosts = await fetch(`${getGatewayUrl()}/tailscale/hosts`).then((r) => r.json());
            const list = (hosts.hosts || []).map(
              (h: any) => `  ${h.tailscaleIp} ${h.hostName} (${(h.vms || []).length} VMs)`
            );
            return `Tailscale Hosts:\n${list.join('\n') || '  None registered'}`;
          });

        case 'storage':
          return gateway(async () => {
            const profile = await apiFetch('/auth/profile');
            const used = profile.storageUsed || 0;
            const quota = profile.storageQuota || 10737418240;
            return `Storage: ${formatSize(used)} / ${formatSize(quota)} (${Math.round((used / quota) * 100)}%)`;
          });

        default:
          return `Unknown command: ${command}. Type "help" for commands.`;
      }
    },
    [cwd, fs, cmdHistory, gateway]
  );

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      const trimmed = input.trim();
      if (!trimmed) return;
      const p = prompt();
      setLines((prev) => [...prev, { kind: 'in', text: `${p} ${trimmed}` }]);
      setCmdHistory((prev) => [...prev, trimmed]);
      try {
        const output = await execute(trimmed);
        if (output) setLines((prev) => [...prev, { kind: 'out', text: output }]);
      } catch (err: any) {
        setLines((prev) => [...prev, { kind: 'err', text: `Error: ${err.message}` }]);
      }
      setInput('');
    },
    [input, execute]
  );

  const colorFor = (kind: 'in' | 'out' | 'err') =>
    kind === 'err' ? C.error : kind === 'in' ? C.text : C.text;

  return (
    <div
      style={{ background: C.bg, color: C.text, height: '100%', display: 'flex', flexDirection: 'column', fontFamily: FONT, fontSize: 13, cursor: 'text' }}
      onClick={() => inputRef.current?.focus()}
    >
      <div ref={scrollRef} style={{ flex: 1, overflow: 'auto', padding: '8px 12px' }}>
        {lines.map((line, i) => (
          <div key={i} style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all', lineHeight: 1.5, color: colorFor(line.kind) }}>
            {line.kind === 'in' ? (
              <span>
                <span style={{ color: C.prompt }}>{prompt()}</span> {line.text.replace(prompt(), '').trim()}
              </span>
            ) : (
              line.text
            )}
          </div>
        ))}
      </div>
      <form
        onSubmit={handleSubmit}
        style={{ display: 'flex', padding: '4px 12px', borderTop: '1px solid #1a1e24' }}
      >
        <span style={{ color: C.prompt, marginRight: 8 }}>{prompt()}</span>
        <input
          ref={inputRef}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          style={{ background: 'none', border: 'none', color: C.text, flex: 1, outline: 'none', fontFamily: FONT, fontSize: 13 }}
          autoFocus
        />
      </form>
    </div>
  );
}
