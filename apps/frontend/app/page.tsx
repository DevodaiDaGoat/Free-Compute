'use client';

import { useEffect, useMemo, useState } from 'react';
import {
  Activity,
  Cable,
  Gamepad2,
  Gauge,
  Monitor,
  RefreshCw,
  Route,
  ShieldCheck,
  TerminalSquare,
  Zap,
} from 'lucide-react';

import type { UniversalProxyCapabilities } from '@free-compute/api-types';

type HealthState = {
  ok: boolean;
  status: number;
  latencyMs: number;
  gatewayUrl: string;
  error?: string;
};

type CapabilitiesState = {
  live: boolean;
  gatewayUrl: string;
  capabilities: UniversalProxyCapabilities;
  error?: string;
};

const modes = [
  { label: 'Desktop', status: 'Ready', icon: Monitor, detail: 'safe stream preset' },
  { label: 'Development', status: 'Ready', icon: TerminalSquare, detail: 'ports and terminal paths' },
  { label: 'Gaming', status: 'GPU preferred', icon: Gamepad2, detail: 'fast stream preset' },
  { label: 'Remote Support', status: 'Guarded', icon: ShieldCheck, detail: 'approval and logs' },
];

const streamStats = [
  { label: 'Target latency', value: '35 ms', tone: 'good' },
  { label: 'Refresh target', value: '120 Hz', tone: 'info' },
  { label: 'Codec path', value: 'H.264', tone: 'warn' },
  { label: 'Input path', value: 'WebRTC / WS', tone: 'good' },
];

const fallbackRoutes = ['web', 'vm-web', 'game-udp', 'rtc'];

export default function Home() {
  const [health, setHealth] = useState<HealthState | null>(null);
  const [capabilities, setCapabilities] = useState<CapabilitiesState | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  async function refresh() {
    setRefreshing(true);
    try {
      const [healthResponse, capabilitiesResponse] = await Promise.all([
        fetch('/api/gateway/health', { cache: 'no-store' }),
        fetch('/api/gateway/capabilities', { cache: 'no-store' }),
      ]);

      setHealth((await healthResponse.json()) as HealthState);
      setCapabilities((await capabilitiesResponse.json()) as CapabilitiesState);
    } finally {
      setRefreshing(false);
    }
  }

  useEffect(() => {
    void refresh();
    const timer = window.setInterval(() => void refresh(), 5000);
    return () => window.clearInterval(timer);
  }, []);

  const protocols = capabilities?.capabilities.protocols ?? [];
  const transports = capabilities?.capabilities.transports ?? [];
  const routeModes = capabilities?.capabilities.routeModes ?? [];
  const gatewayOnline = Boolean(health?.ok && capabilities?.live);

  const routeRows = useMemo(() => {
    const pathSets = capabilities?.capabilities.clientPaths;
    if (!pathSets) {
      return [];
    }

    return Object.entries(pathSets).map(([client, paths]) => ({
      client,
      paths: Object.entries(paths)
        .filter(([, value]) => Boolean(value))
        .map(([key, value]) => (value === true ? key : String(value))),
    }));
  }, [capabilities]);

  return (
    <main className="console-shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">FreeCompute</p>
          <h1>Remote Operations Console</h1>
        </div>
        <div className="topbar-actions">
          <div className={gatewayOnline ? 'status-pill online' : 'status-pill offline'}>
            <span />
            {gatewayOnline ? 'Gateway online' : 'Gateway offline'}
          </div>
          <button className="icon-button" onClick={() => void refresh()} disabled={refreshing} aria-label="Refresh gateway status">
            <RefreshCw size={18} className={refreshing ? 'spin' : ''} />
            <span>Refresh</span>
          </button>
        </div>
      </header>

      <section className="overview-grid">
        <div className="panel live-panel">
          <div className="panel-heading">
            <div>
              <p className="eyebrow">Gateway</p>
              <h2>{health?.gatewayUrl ?? 'http://127.0.0.1:8080'}</h2>
            </div>
            <Gauge size={22} />
          </div>
          <div className="metric-row">
            <span>Status</span>
            <strong>{health ? `${health.status || 'down'}` : 'checking'}</strong>
          </div>
          <div className="metric-row">
            <span>Latency</span>
            <strong>{health ? `${health.latencyMs} ms` : '...'}</strong>
          </div>
          <div className="route-strip">
            {(protocols.length ? protocols : ['http', 'https', 'tcp', 'udp', 'ssh', 'webrtc']).map((protocol) => (
              <span key={protocol}>{protocol}</span>
            ))}
          </div>
        </div>

        <div className="panel stream-panel">
          <div className="panel-heading">
            <div>
              <p className="eyebrow">Streaming</p>
              <h2>Fast path monitor</h2>
            </div>
            <Zap size={22} />
          </div>
          <div className="signal-stage" aria-hidden="true">
            <span className="node client" />
            <span className="pulse one" />
            <span className="pulse two" />
            <span className="node host" />
          </div>
          <div className="stat-grid">
            {streamStats.map((stat) => (
              <div className={`stat ${stat.tone}`} key={stat.label}>
                <span>{stat.label}</span>
                <strong>{stat.value}</strong>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section className="mode-grid" aria-label="Session modes">
        {modes.map((mode) => {
          const Icon = mode.icon;
          return (
            <div className="mode-panel" key={mode.label}>
              <Icon size={22} />
              <div>
                <h3>{mode.label}</h3>
                <p>{mode.status}</p>
              </div>
              <span>{mode.detail}</span>
            </div>
          );
        })}
      </section>

      <section className="wide-grid">
        <div className="panel">
          <div className="panel-heading">
            <div>
              <p className="eyebrow">Universal Proxy</p>
              <h2>Client paths</h2>
            </div>
            <Route size={22} />
          </div>
          <div className="path-table">
            {routeRows.map((row) => (
              <div className="path-row" key={row.client}>
                <strong>{row.client}</strong>
                <div>
                  {row.paths.map((path) => (
                    <code key={path}>{path}</code>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className="panel">
          <div className="panel-heading">
            <div>
              <p className="eyebrow">VM Tunnel</p>
              <h2>Local route set</h2>
            </div>
            <Cable size={22} />
          </div>
          <div className="route-list">
            {fallbackRoutes.map((route) => (
              <div className="route-item" key={route}>
                <span>{route}</span>
                <code>{route === 'vm-web' ? '/ws/vm-web' : route === 'rtc' ? '/signal/rtc/rooms/demo' : route}</code>
              </div>
            ))}
          </div>
          <div className="transport-list">
            {transports.map((transport) => (
              <span key={transport}>{transport}</span>
            ))}
          </div>
        </div>

        <div className="panel edge-panel">
          <div className="panel-heading">
            <div>
              <p className="eyebrow">Edge</p>
              <h2>BunnyCDN profile</h2>
            </div>
            <Activity size={22} />
          </div>
          <div className="route-list">
            {(capabilities?.capabilities.bunnyCdn.bypassCache ?? ['/proxy/*', '/ws/*', '/connect/*', '/agent/*', '/signal/*']).map(
              (path) => (
                <div className="route-item" key={path}>
                  <span>Bypass</span>
                  <code>{path}</code>
                </div>
              ),
            )}
          </div>
          <div className="transport-list">
            {routeModes.map((mode) => (
              <span key={mode}>{mode}</span>
            ))}
          </div>
        </div>
      </section>
    </main>
  );
}
