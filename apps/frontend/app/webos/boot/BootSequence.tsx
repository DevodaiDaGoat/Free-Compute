'use client';

import { useCallback, useEffect, useState } from 'react';

interface Props {
  phase: 'bios' | 'loading' | 'login';
  onPhaseChange: (p: 'bios' | 'loading' | 'login' | 'desktop') => void;
  onBootComplete: () => void;
}

const GATEWAY = process.env.NEXT_PUBLIC_GATEWAY_URL || 'http://localhost:8080';

interface AuthTokens {
  accessToken: string;
  refreshToken?: string;
  expiresAt: string;
}

interface UserProfile {
  id: string;
  email: string;
  displayName: string;
  storageUsed: number;
  storageQuota: number;
  tailscaleIp?: string;
}

let currentTokens: AuthTokens | null = null;
let currentUser: UserProfile | null = null;

export function getTokens() { return currentTokens; }
export function getUser() { return currentUser; }
export function getGatewayUrl() { return GATEWAY; }

export async function apiFetch(path: string, options?: RequestInit) {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (currentTokens) {
    headers['Authorization'] = `Bearer ${currentTokens.accessToken}`;
  }
  const res = await fetch(`${GATEWAY}${path}`, { ...options, headers: { ...headers, ...options?.headers } });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export default function BootSequence({ phase, onPhaseChange, onBootComplete }: Props) {
  const [progress, setProgress] = useState(0);
  const [logs, setLogs] = useState<string[]>([]);

  useEffect(() => {
    if (phase === 'bios') {
      setLogs(['FreeCompute WebOS v0.1.0', 'Initializing kernel...', 'Starting system services...']);
      const t = setTimeout(() => onPhaseChange('loading'), 1500);
      return () => clearTimeout(t);
    }
  }, [phase, onPhaseChange]);

  useEffect(() => {
    if (phase === 'loading') {
      bootSteps();
    }
  }, [phase]);

  async function bootSteps() {
    const steps = [
      { msg: 'Mounting virtual filesystem...', p: 15 },
      { msg: 'Loading network stack...', p: 30 },
      { msg: 'Connecting to gateway...', p: 45 },
      { msg: 'Establishing Tailscale tunnel...', p: 60 },
      { msg: 'Starting window manager...', p: 75 },
      { msg: 'Loading user apps...', p: 90 },
      { msg: 'Ready', p: 100 },
    ];
    for (const step of steps) {
      await new Promise((r) => setTimeout(r, 400));
      setLogs((prev) => [...prev.slice(-4), step.msg]);
      setProgress(step.p);
    }
    await new Promise((r) => setTimeout(r, 300));
    onBootComplete();
  }

  if (phase === 'login') {
    return <LoginScreen onLogin={() => onBootComplete()} />;
  }

  return (
    <div style={{ background: '#0a0a0a', color: '#18e2ff', height: '100vh', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', fontFamily: 'monospace', padding: 24 }}>
      <pre style={{ fontSize: 11, lineHeight: 1.2, marginBottom: 32 }}>{`
  ███████╗██████╗ ███████╗███████╗ ██████╗ ██████╗ ███╗   ███╗██████╗ ██╗   ██╗████████╗███████╗
  ██╔════╝██╔══██╗██╔════╝██╔════╝██╔════╝██╔═══██╗████╗ ████║██╔══██╗██║   ██║╚══██╔══╝██╔════╝
  █████╗  ██████╔╝█████╗  █████╗  ██║     ██║   ██║██╔████╔██║██████╔╝██║   ██║   ██║   █████╗
  ██╔══╝  ██╔══██╗██╔══╝  ██╔══╝  ██║     ██║   ██║██║╚██╔╝██║██╔═══╝ ██║   ██║   ██║   ██╔══╝
  ██║     ██║  ██║███████╗██║     ╚██████╗╚██████╔╝██║ ╚═╝ ██║██║     ╚██████╔╝   ██║   ███████╗
  ╚═╝     ╚═╝  ╚═╝╚══════╝╚═╝      ╚═════╝ ╚═════╝ ╚═╝     ╚═╝╚═╝      ╚═════╝    ╚═╝   ╚══════╝
      `}</pre>
      <div style={{ width: 400, maxWidth: '90vw' }}>
        <div style={{ height: 2, background: '#1a3a3a', borderRadius: 2, overflow: 'hidden' }}>
          <div style={{ height: '100%', width: `${progress}%`, background: '#18e2ff', transition: 'width 0.3s' }} />
        </div>
      </div>
      <div style={{ marginTop: 16, fontSize: 12, color: '#6af' }}>
        {logs.map((l, i) => <div key={i}>{l}</div>)}
      </div>
    </div>
  );
}

function LoginScreen({ onLogin }: { onLogin: () => void }) {
  const [isRegister, setIsRegister] = useState(false);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [name, setName] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      const endpoint = isRegister ? '/auth/register' : '/auth/login';
      const body = isRegister ? { email, password, displayName: name } : { email, password };
      const data = await apiFetch(endpoint, { method: 'POST', body: JSON.stringify(body) });
      currentTokens = data.tokens;
      currentUser = data.user;
      onLogin();
    } catch (err: any) {
      setError(err.message || 'Authentication failed');
    } finally {
      setLoading(false);
    }
  }, [isRegister, email, password, name, onLogin]);

  return (
    <div style={{ background: '#0f1923', color: '#fff', height: '100vh', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', fontFamily: 'system-ui, sans-serif', gap: 16 }}>
      <div style={{ fontSize: 48, fontWeight: 200, letterSpacing: 4 }}>FreeCompute</div>
      <div style={{ fontSize: 14, color: '#888', marginBottom: 16 }}>WebOS Desktop</div>
      <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 12, width: 280 }}>
        {isRegister && (
          <input
            type="text"
            placeholder="Display Name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            style={{ padding: '10px 16px', background: '#1a1a2e', border: '1px solid #333', borderRadius: 6, color: '#fff', fontSize: 14 }}
          />
        )}
        <input
          type="email"
          placeholder="Email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          style={{ padding: '10px 16px', background: '#1a1a2e', border: '1px solid #333', borderRadius: 6, color: '#fff', fontSize: 14 }}
          required
        />
        <input
          type="password"
          placeholder="Password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          style={{ padding: '10px 16px', background: '#1a1a2e', border: '1px solid #333', borderRadius: 6, color: '#fff', fontSize: 14 }}
          required
        />
        {error && <div style={{ color: '#f44', fontSize: 12 }}>{error}</div>}
        <button
          type="submit"
          disabled={loading}
          style={{ padding: '10px 32px', background: '#18e2ff', border: 'none', borderRadius: 6, color: '#000', fontWeight: 600, cursor: 'pointer', fontSize: 14, opacity: loading ? 0.6 : 1 }}
        >
          {loading ? '...' : isRegister ? 'Create Account' : 'Sign In'}
        </button>
        <button
          type="button"
          onClick={() => { setIsRegister(!isRegister); setError(''); }}
          style={{ background: 'none', border: 'none', color: '#6af', cursor: 'pointer', fontSize: 12 }}
        >
          {isRegister ? 'Already have an account? Sign in' : "Don't have an account? Register"}
        </button>
      </form>
    </div>
  );
}
