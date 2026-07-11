'use client';

import { memo, useCallback, useEffect, useRef, useState } from 'react';
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
  Wifi,
  Zap,
} from 'lucide-react';

const modes = [
  { label: 'Desktop', status: 'Ready', icon: Monitor, detail: 'safe stream preset' },
  { label: 'Development', status: 'Ready', icon: TerminalSquare, detail: 'ports and terminal paths' },
  { label: 'Gaming', status: 'GPU preferred', icon: Gamepad2, detail: 'fast stream preset' },
  { label: 'Remote Support', status: 'Guarded', icon: ShieldCheck, detail: 'approval and logs' },
];

const fallbackRoutes = ['web', 'vm-web', 'game-udp', 'rtc'];

type GatewayState = {
  ok: boolean;
  status: number;
  live: boolean;
  protocols: string[];
  transports: string[];
  routeModes: string[];
  rtt: number | null;
  rttHistory: { ts: number; rtt: number }[];
};

function useGatewayStatus() {
  const [state, setState] = useState<GatewayState>({
    ok: false,
    status: 0,
    live: false,
    protocols: [],
    transports: [],
    routeModes: [],
    rtt: null,
    rttHistory: [],
  });

  const stateRef = useRef(state);
  stateRef.current = state;

  useEffect(() => {
    let es: EventSource | null = null;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let paused = false;
    let retries = 0;

    const connect = () => {
      if (paused) return;
      es = new EventSource('/api/gateway/events');

      es.addEventListener('health', (e) => {
        const h = JSON.parse(e.data);
        retries = 0;
        setState((prev) => {
          const rtt = h.rtt ?? null;
          const history = [...prev.rttHistory];
          if (rtt !== null) {
            history.push({ ts: Date.now(), rtt });
            if (history.length > 60) history.shift();
          }
          return { ...prev, ok: h.ok, status: h.status, rtt, rttHistory: history };
        });
      });

      es.addEventListener('capabilities', (e) => {
        const c = JSON.parse(e.data);
        setState((prev) => ({
          ...prev,
          live: c.live,
          protocols: c.protocols ?? prev.protocols,
          transports: c.transports ?? prev.transports,
          routeModes: c.routeModes ?? prev.routeModes,
        }));
      });

      es.onerror = () => {
        es?.close();
        retries++;
        const delay = Math.min(1000 * Math.pow(1.5, retries), 15000);
        timer = setTimeout(connect, delay);
      };
    };

    const onVisibility = () => {
      if (document.hidden) {
        paused = true;
        es?.close();
        if (timer) { clearTimeout(timer); timer = null; }
      } else {
        paused = false;
        retries = 0;
        connect();
      }
    };

    document.addEventListener('visibilitychange', onVisibility);
    connect();

    return () => {
      es?.close();
      if (timer) clearTimeout(timer);
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, []);

  return state;
}

function RttIndicator({ rtt, rttHistory }: { rtt: number | null; rttHistory: { ts: number; rtt: number }[] }) {
  const quality = rtt === null ? 'unknown' : rtt < 30 ? 'excellent' : rtt < 80 ? 'good' : rtt < 150 ? 'degraded' : 'poor';
  const qualityColors: Record<string, string> = {
    excellent: '#198754',
    good: '#246bfe',
    degraded: '#a96800',
    poor: '#b42318',
    unknown: '#607065',
  };

  const minDim = Math.min(...rttHistory.map(h => h.rtt), rtt ?? 0);
  const maxDim = Math.max(...rttHistory.map(h => h.rtt), rtt ?? 1);
  const range = Math.max(maxDim - minDim, 1);
  const width = 140;
  const height = 32;
  const points = rttHistory.length < 2 ? [] : rttHistory.map((h, i) => {
    const x = (i / Math.max(rttHistory.length - 1, 1)) * width;
    const y = height - ((h.rtt - minDim) / range) * height;
    return `${x},${y}`;
  });

  return (
    <div className="panel-heading rtt-panel">
      <div>
        <p className="eyebrow">Connection</p>
        <h2 style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <Wifi size={20} color={qualityColors[quality]} />
          <span style={{ color: qualityColors[quality] }}>
            {rtt === null ? '--' : `${Math.round(rtt)} ms`}
          </span>
        </h2>
      </div>
      {points.length > 1 && (
        <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} className="rtt-sparkline">
          <polyline
            fill="none"
            stroke={qualityColors[quality]}
            strokeWidth="1.5"
            points={points.join(' ')}
          />
        </svg>
      )}
    </div>
  );
}

function ModePanel({ label, status, icon: Icon, detail }: typeof modes[0]) {
  return (
    <div className="mode-panel" onMouseEnter={() => {
      const link = document.createElement('link');
      link.rel = 'preconnect';
      link.href = process.env.NEXT_PUBLIC_GATEWAY_URL || 'http://localhost:8080';
      document.head.appendChild(link);
      setTimeout(() => link.remove(), 10000);
    }}>
      <Icon size={22} />
      <div>
        <h3>{label}</h3>
        <p>{status}</p>
      </div>
      <span>{detail}</span>
    </div>
  );
}

const ModePanelMemo = memo(ModePanel);

function GatewayPanel({ ok, status, protocols, rtt }: GatewayState) {
  const color = ok ? (rtt !== null && rtt > 150 ? '#a96800' : '#198754') : '#b42318';
  return (
    <div className="panel live-panel">
      <div className="panel-heading">
        <div>
          <p className="eyebrow">Gateway</p>
          <h2>Console</h2>
        </div>
        <Gauge size={22} color={color} />
      </div>
      <div className="metric-row">
        <span>Status</span>
        <strong style={{ color }}>{ok ? `${status || 'ok'}` : 'offline'}</strong>
      </div>
      <div className="metric-row">
        <span>Connection</span>
        <strong style={{ color }}>{ok ? 'Live' : 'Disconnected'}</strong>
      </div>
      {rtt !== null && (
        <div className="metric-row">
          <span>Latency</span>
          <strong>{Math.round(rtt)} ms</strong>
        </div>
      )}
      <div className="route-strip">
        {(protocols.length ? protocols : ['http', 'https', 'tcp', 'udp', 'ssh', 'webrtc']).map((protocol) => (
          <span key={protocol}>{protocol}</span>
        ))}
      </div>
    </div>
  );
}

const GatewayPanelMemo = memo(GatewayPanel);

export default function GatewayPage() {
  const gateway = useGatewayStatus();
  const [manualRefreshing, setManualRefreshing] = useState(false);

  const gatewayOnline = gateway.ok && gateway.live;

  useEffect(() => {
    if (gatewayOnline) {
      const link = document.createElement('link');
      link.rel = 'preconnect';
      link.href = process.env.NEXT_PUBLIC_GATEWAY_URL || 'http://localhost:8080';
      document.head.appendChild(link);
      return () => { link.remove(); };
    }
  }, [gatewayOnline]);

  const handleManualRefresh = useCallback(async () => {
    setManualRefreshing(true);
    await Promise.allSettled([
      fetch('/api/gateway/health', { cache: 'no-store' }),
      fetch('/api/gateway/capabilities', { cache: 'no-store' }),
    ]);
    setManualRefreshing(false);
  }, []);

  return (
    <div style={{ background: '#0d1117', color: '#c9d1d9', minHeight: '100vh' }}>
      <style>{`
        .panel { background: #161b22 !important; border-color: #30363d !important; }
        .console-shell { background: #0d1117 !important; }
        .topbar { background: #161b22 !important; border-bottom-color: #30363d !important; }
      `}</style>
    <main className="console-shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">FreeCompute</p>
          <h1>Gateway Console</h1>
        </div>
        <div className="topbar-actions">
          <div className={gatewayOnline ? 'status-pill online' : 'status-pill offline'}>
            <span />
            {gatewayOnline ? 'Gateway online' : 'Gateway offline'}
          </div>
          <button className="icon-button" onClick={handleManualRefresh} disabled={manualRefreshing} aria-label="Refresh gateway status">
            <RefreshCw size={18} className={manualRefreshing ? 'spin' : ''} />
            <span>Refresh</span>
          </button>
        </div>
      </header>

      <section className="overview-grid">
        <GatewayPanelMemo {...gateway} />

        <div className="panel stream-panel">
          <div className="panel-heading">
            <div>
              <p className="eyebrow">Streaming</p>
              <h2>Fast path monitor</h2>
            </div>
            <Zap size={22} />
          </div>
          <RttIndicator rtt={gateway.rtt} rttHistory={gateway.rttHistory} />
          <div className="signal-stage" aria-hidden="true">
            <span className="node client" />
            <span className="pulse one" />
            <span className="pulse two" />
            <span className="node host" />
          </div>
        </div>
      </section>

      <section className="mode-grid" aria-label="Session modes">
        {modes.map((mode) => (
          <ModePanelMemo key={mode.label} {...mode} />
        ))}
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
            <div className="path-row">
              <strong>browser</strong>
              <div>
                <code>/proxy/{'{routeID}'}/{'{path}'}</code>
                <code>/ws/{'{routeID}'}</code>
                <code>/signal/{'{routeID}'}/rooms/{'{roomID}'}</code>
              </div>
            </div>
            <div className="path-row">
              <strong>webos-app</strong>
              <div>
                <code>/proxy/{'{routeID}'}/{'{path}'}</code>
                <code>/connect/{'{routeID}'}</code>
                <code>/ws/{'{routeID}'}</code>
              </div>
            </div>
            <div className="path-row">
              <strong>host-agent</strong>
              <div>
                <code>/agent/{'{routeID}'}</code>
              </div>
            </div>
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
            {gateway.transports.map((t) => (
              <span key={t}>{t}</span>
            ))}
          </div>
        </div>

        <div className="panel edge-panel">
          <div className="panel-heading">
            <div>
              <p className="eyebrow">Edge</p>
              <h2>CDN profile</h2>
            </div>
            <Activity size={22} />
          </div>
          <div className="route-list">
            {['/proxy/*', '/ws/*', '/connect/*', '/agent/*', '/signal/*'].map(
              (path) => (
                <div className="route-item" key={path}>
                  <span>Bypass</span>
                  <code>{path}</code>
                </div>
              ),
            )}
          </div>
          <div className="transport-list">
            {gateway.routeModes.map((mode) => (
              <span key={mode}>{mode}</span>
            ))}
          </div>
        </div>
      </section>
    </main>
    </div>
  );
}
