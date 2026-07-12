'use client';

import { useCallback, useEffect, useState } from 'react';

interface Props {
  phase: 'bios' | 'loading' | 'login' | 'desktop';
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

// Restore from sessionStorage on module load so a page reload doesn't drop
// the auth state (previously any refresh silently logged the user out into
// demo mode). We keep the module-scoped variable as the primary source of
// truth; sessionStorage is a mirror that survives navigation between /webos
// and /connect and lets api/websocket.ts pick up the token without importing
// this file (which would create a circular dep).
if (typeof window !== 'undefined') {
  try {
    const rawTokens = window.sessionStorage.getItem('freecompute:tokens');
    if (rawTokens) currentTokens = JSON.parse(rawTokens);
    const rawUser = window.sessionStorage.getItem('freecompute:user');
    if (rawUser) currentUser = JSON.parse(rawUser);
  } catch {
    /* corrupt storage — ignore and start fresh */
  }
}

function persistAuth() {
  if (typeof window === 'undefined') return;
  try {
    if (currentTokens) window.sessionStorage.setItem('freecompute:tokens', JSON.stringify(currentTokens));
    else window.sessionStorage.removeItem('freecompute:tokens');
    if (currentUser) window.sessionStorage.setItem('freecompute:user', JSON.stringify(currentUser));
    else window.sessionStorage.removeItem('freecompute:user');
  } catch {
    /* quota / disabled storage — ignore */
  }
}

export function getTokens() { return currentTokens; }
export function getUser() { return currentUser; }
export function getGatewayUrl() { return GATEWAY; }
export function clearAuth() {
  currentTokens = null;
  currentUser = null;
  persistAuth();
}

export async function apiFetch(path: string, options?: RequestInit) {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (currentTokens) {
    headers['Authorization'] = `Bearer ${currentTokens.accessToken}`;
  }
  const res = await fetch(`${GATEWAY}${path}`, { ...options, headers: { ...headers, ...options?.headers } });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

// --- Fake data so the WebOS boots convincingly without a live backend ---
const GB = 1024 * 1024 * 1024;
const FAKE_STORAGE_USED = 2.4 * GB;
const FAKE_STORAGE_QUOTA = 10 * GB;

function formatGB(bytes: number) {
  return `${(bytes / GB).toFixed(1)} GB`;
}

export default function BootSequence({ phase, onPhaseChange, onBootComplete }: Props) {
  const [progress, setProgress] = useState(0);
  const [logs, setLogs] = useState<string[]>([]);

  useEffect(() => {
    if (phase === 'bios') {
      const lines = [
        'FreeCompute WebOS v0.1.0',
        'BIOS POST ......................... OK',
        'Detecting virtual hardware ........ OK',
        'Initializing kernel ...............',
        'Starting system services .........',
      ];
      setLogs([]);
      let i = 0;
      const interval = setInterval(() => {
        setLogs((prev) => [...prev, lines[i]]);
        i += 1;
        if (i >= lines.length) clearInterval(interval);
      }, 260);
      const t = setTimeout(() => onPhaseChange('loading'), 1600);
      return () => {
        clearInterval(interval);
        clearTimeout(t);
      };
    }
    if (phase === 'loading') {
      let cancelled = false;
      (async () => {
        const steps = [
          { msg: 'Mounting virtual filesystem...', p: 15 },
          { msg: 'Loading network stack...', p: 30 },
          { msg: 'Connecting to gateway...', p: 45 },
          { msg: 'Establishing Tailscale tunnel...', p: 60 },
          { msg: 'Starting window manager...', p: 75 },
          { msg: 'Loading user apps...', p: 90 },
          { msg: 'Ready', p: 100 },
        ];
        setProgress(0);
        setLogs([]);
        for (const step of steps) {
          await new Promise((r) => setTimeout(r, 220));
          if (cancelled) return;
          setLogs((prev) => [...prev.slice(-4), step.msg]);
          setProgress(step.p);
        }
        await new Promise((r) => setTimeout(r, 350));
        if (!cancelled) onPhaseChange('login');
      })();
      return () => {
        cancelled = true;
      };
    }
  }, [phase, onPhaseChange]);

  if (phase === 'login') {
    return <LoginScreen onLogin={onBootComplete} />;
  }

  return (
    <div
      style={{
        background: 'radial-gradient(circle at 50% 40%, #0d1420 0%, #060a10 70%, #030507 100%)',
        color: '#18e2ff',
        height: '100vh',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        fontFamily: '"SF Mono", "JetBrains Mono", Menlo, Consolas, monospace',
        padding: 24,
      }}
    >
      <Logo />

      {phase === 'loading' && (
        <div style={{ width: 420, maxWidth: '90vw', marginTop: 40 }}>
          <div
            style={{
              height: 4,
              background: 'rgba(24,226,255,0.12)',
              borderRadius: 4,
              overflow: 'hidden',
            }}
          >
            <div
              style={{
                height: '100%',
                width: `${progress}%`,
                background: 'linear-gradient(90deg, #0ea5c4, #18e2ff)',
                boxShadow: '0 0 12px rgba(24,226,255,0.7)',
                transition: 'width 0.3s ease',
              }}
            />
          </div>
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              marginTop: 8,
              fontSize: 11,
              color: '#4a7a8a',
            }}
          >
            <span>booting</span>
            <span>{progress}%</span>
          </div>
        </div>
      )}

      <div
        style={{
          marginTop: 24,
          fontSize: 12,
          lineHeight: 1.7,
          color: '#5fa8bd',
          minHeight: 120,
          width: 420,
          maxWidth: '90vw',
        }}
      >
        {logs.map((l, i) => (
          <div key={i} style={{ opacity: 0.55 + (i / Math.max(logs.length, 1)) * 0.45 }}>
            <span style={{ color: '#2c6a7a', marginRight: 8 }}>›</span>
            {l}
          </div>
        ))}
      </div>
    </div>
  );
}

function Logo() {
  return (
    <div style={{ textAlign: 'center', userSelect: 'none' }}>
      <div
        style={{
          fontFamily: '"SF Mono", "JetBrains Mono", Menlo, Consolas, monospace',
          fontSize: 44,
          fontWeight: 600,
          letterSpacing: 6,
          color: '#eafcff',
          textShadow: '0 0 18px rgba(24,226,255,0.55), 0 0 40px rgba(24,226,255,0.25)',
        }}
      >
        Free<span style={{ color: '#18e2ff' }}>Compute</span>
      </div>
      <div
        style={{
          marginTop: 10,
          fontSize: 12,
          letterSpacing: 4,
          color: '#3d6a78',
          textTransform: 'uppercase',
        }}
      >
        WebOS &middot; v0.1.0
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

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      setLoading(true);
      setError('');
      try {
        const endpoint = isRegister ? '/auth/register' : '/auth/login';
        const body = isRegister ? { email, password, displayName: name } : { email, password };
        const data = await apiFetch(endpoint, { method: 'POST', body: JSON.stringify(body) });
        // Defend against malformed backend responses: a 2xx with missing tokens
        // or user (e.g. proxy stripping the body) previously stored `undefined`
        // in sessionStorage and left the app in a half-authenticated state.
        if (!data || !data.tokens || !data.tokens.accessToken || !data.user) {
          throw new Error('Server returned an incomplete auth response');
        }
        currentTokens = data.tokens;
        currentUser = data.user;
        persistAuth();
        onLogin();
      } catch (err: any) {
        // Distinguish backend-rejected credentials (surface as an error) from
        // an unreachable gateway (fall back to demo mode). Previously ANY
        // failure silently logged the user in as a demo user, hiding wrong-
        // password prompts and misleading users about their real account.
        const msg = String(err?.message || err || '');
        const looksLikeAuthFailure =
          /HTTP\s*4\d\d/i.test(msg) ||
          /\bunauthorized|forbidden|invalid|not found|already exists|user\s*exists/i.test(msg);
        let gatewayReachable = false;
        try {
          const ping = await fetch(`${GATEWAY}/healthz`, { method: 'GET' });
          gatewayReachable = ping.ok || ping.status < 500;
        } catch {
          gatewayReachable = false;
        }
        if (gatewayReachable || looksLikeAuthFailure) {
          setError(isRegister ? 'Registration failed. Check your input and try again.' : 'Sign-in failed. Check your email and password.');
          setLoading(false);
          return;
        }
        // No backend at all — keep the demo UX so devs can explore the
        // WebOS offline. Real deployments will always hit the gateway path
        // above and see proper errors.
        await new Promise((r) => setTimeout(r, 400));
        currentTokens = {
          accessToken: 'demo-access-token',
          refreshToken: 'demo-refresh-token',
          expiresAt: new Date(Date.now() + 3600_000).toISOString(),
        };
        currentUser = {
          id: 'demo-user',
          email: email || 'demo@freecompute.dev',
          displayName: name || (email ? email.split('@')[0] : 'Demo User'),
          storageUsed: FAKE_STORAGE_USED,
          storageQuota: FAKE_STORAGE_QUOTA,
        };
        persistAuth();
        onLogin();
      } finally {
        setLoading(false);
      }
    },
    [isRegister, email, password, name, onLogin]
  );

  const usedPct = Math.min(100, (FAKE_STORAGE_USED / FAKE_STORAGE_QUOTA) * 100);

  const inputStyle: React.CSSProperties = {
    padding: '12px 14px',
    background: '#0e1622',
    border: '1px solid #22303f',
    borderRadius: 8,
    color: '#e8eef4',
    fontSize: 14,
    outline: 'none',
    width: '100%',
    boxSizing: 'border-box',
  };

  return (
    <div
      style={{
        background: 'radial-gradient(circle at 50% 30%, #101a26 0%, #0b111a 60%, #070b11 100%)',
        color: '#e8eef4',
        height: '100vh',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        fontFamily: 'system-ui, -apple-system, Segoe UI, sans-serif',
        padding: 24,
      }}
    >
      <div
        style={{
          width: 360,
          maxWidth: '92vw',
          background: 'rgba(16,24,35,0.72)',
          border: '1px solid #1f2c3a',
          borderRadius: 16,
          padding: 32,
          boxShadow: '0 24px 60px rgba(0,0,0,0.55)',
          backdropFilter: 'blur(12px)',
        }}
      >
        <div style={{ textAlign: 'center', marginBottom: 28 }}>
          <div
            style={{
              fontFamily: '"SF Mono", "JetBrains Mono", Menlo, Consolas, monospace',
              fontSize: 28,
              fontWeight: 600,
              letterSpacing: 3,
              color: '#eafcff',
              textShadow: '0 0 16px rgba(24,226,255,0.4)',
            }}
          >
            Free<span style={{ color: '#18e2ff' }}>Compute</span>
          </div>
          <div style={{ fontSize: 13, color: '#6b7d8f', marginTop: 6 }}>
            {isRegister ? 'Create your account' : 'Sign in to your desktop'}
          </div>
        </div>

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          {isRegister && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              <label style={{ fontSize: 12, color: '#8a9bab' }}>Display Name</label>
              <input
                type="text"
                placeholder="Jane Doe"
                value={name}
                onChange={(e) => setName(e.target.value)}
                style={inputStyle}
              />
            </div>
          )}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <label style={{ fontSize: 12, color: '#8a9bab' }}>Email</label>
            <input
              type="email"
              placeholder="you@example.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              style={inputStyle}
              required
            />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <label style={{ fontSize: 12, color: '#8a9bab' }}>Password</label>
            <input
              type="password"
              placeholder="••••••••"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              style={inputStyle}
              required
            />
          </div>

          {error && <div style={{ color: '#ff6b6b', fontSize: 12 }}>{error}</div>}

          <button
            type="submit"
            disabled={loading}
            style={{
              marginTop: 4,
              padding: '12px 32px',
              background: loading ? '#0e6d80' : 'linear-gradient(90deg, #0ea5c4, #18e2ff)',
              border: 'none',
              borderRadius: 8,
              color: '#04222a',
              fontWeight: 700,
              cursor: loading ? 'default' : 'pointer',
              fontSize: 14,
              letterSpacing: 0.5,
              transition: 'opacity 0.2s',
              opacity: loading ? 0.7 : 1,
            }}
          >
            {loading ? 'Signing in…' : isRegister ? 'Create Account' : 'Sign In'}
          </button>

          <button
            type="button"
            onClick={() => {
              setIsRegister(!isRegister);
              setError('');
            }}
            style={{
              background: 'none',
              border: 'none',
              color: '#5fa8bd',
              cursor: 'pointer',
              fontSize: 12,
              marginTop: 2,
            }}
          >
            {isRegister ? 'Already have an account? Sign in' : "Don't have an account? Register"}
          </button>
        </form>

        {/* Storage quota — realistic fake data shown even without a backend */}
        <div
          style={{
            marginTop: 24,
            paddingTop: 20,
            borderTop: '1px solid #1f2c3a',
          }}
        >
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              marginBottom: 8,
            }}
          >
            <span style={{ fontSize: 12, color: '#8a9bab' }}>Storage</span>
            <span style={{ fontSize: 12, color: '#c7d3de' }}>
              {formatGB(FAKE_STORAGE_USED)} used of {formatGB(FAKE_STORAGE_QUOTA)}
            </span>
          </div>
          <div
            style={{
              height: 8,
              background: '#0e1622',
              border: '1px solid #22303f',
              borderRadius: 6,
              overflow: 'hidden',
            }}
          >
            <div
              style={{
                height: '100%',
                width: `${usedPct}%`,
                minWidth: 6,
                background: 'linear-gradient(90deg, #0ea5c4, #18e2ff)',
                boxShadow: '0 0 8px rgba(24,226,255,0.5)',
              }}
            />
          </div>
          <div style={{ fontSize: 11, color: '#5a6b7c', marginTop: 6 }}>
            {usedPct.toFixed(1)}% of your quota in use
          </div>
        </div>
      </div>
    </div>
  );
}
