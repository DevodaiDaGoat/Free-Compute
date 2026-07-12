'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Monitor, Play, Square, Wifi, Loader2, Terminal as TerminalIcon,
  Radio, RefreshCw, Copy, Check,
} from 'lucide-react';
import { getGatewayUrl, getTokens } from '../../boot/BootSequence';
import { useTunnelConnection, CH_VIDEO, CH_AUDIO, CH_SSH } from '../../system/hooks/useTunnelConnection';
import type { TunnelStatus } from '../../system/hooks/useTunnelConnection';

type Mode = 'desktop' | 'development' | 'gaming' | 'remote-support';
type Preset = 'safe' | 'fast';
type Tab = 'stream' | 'ssh' | 'tailscale';

function Tab2({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      style={{
        padding: '6px 14px',
        borderRadius: 6,
        border: 'none',
        background: active ? 'rgba(88,166,255,0.15)' : 'transparent',
        color: active ? '#58a6ff' : '#6e7681',
        cursor: 'pointer',
        fontSize: 12,
        fontWeight: 700,
        borderBottom: active ? '2px solid #58a6ff' : '2px solid transparent',
      }}
    >
      {label}
    </button>
  );
}

const STATUS_COLOR: Record<TunnelStatus, string> = {
  idle: '#6e7681',
  connecting: '#246bfe',
  connected: '#3fb950',
  reconnecting: '#d29922',
  closed: '#6e7681',
  error: '#f85149',
};

function StreamPanel({
  mode, setMode, preset, setPreset, gpu, setGpu,
  sessionId, sessionState, connecting, rttMs, tunnelStatus,
  onConnect, onDisconnect,
  videoRef, log,
}: {
  mode: Mode; setMode: (m: Mode) => void;
  preset: Preset; setPreset: (p: Preset) => void;
  gpu: boolean; setGpu: (g: boolean) => void;
  sessionId: string | null; sessionState: string;
  connecting: boolean; rttMs: number; tunnelStatus: TunnelStatus;
  onConnect: () => void; onDisconnect: () => void;
  videoRef: React.RefObject<HTMLVideoElement | null>;
  log: string[];
}) {
  const MODES: { key: Mode; label: string }[] = [
    { key: 'desktop', label: 'Desktop' },
    { key: 'development', label: 'Dev' },
    { key: 'gaming', label: 'Gaming' },
    { key: 'remote-support', label: 'Support' },
  ];
  return (
    <div style={{ flex: 1, overflow: 'auto', padding: 14, display: 'flex', flexDirection: 'column', gap: 12 }}>
      {/* Config row */}
      {!sessionId && (
        <>
          <div>
            <div style={{ fontSize: 11, color: '#6e7681', marginBottom: 6, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em' }}>Mode</div>
            <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
              {MODES.map((m) => (
                <button key={m.key} onClick={() => setMode(m.key)} style={{ padding: '6px 14px', borderRadius: 6, border: `1px solid ${mode === m.key ? '#58a6ff' : '#21262d'}`, background: mode === m.key ? 'rgba(88,166,255,0.12)' : 'rgba(255,255,255,0.03)', color: mode === m.key ? '#58a6ff' : '#8b949e', cursor: 'pointer', fontSize: 12, fontWeight: 600 }}>
                  {m.label}
                </button>
              ))}
            </div>
          </div>
          <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
            {(['safe', 'fast'] as Preset[]).map((p) => (
              <button key={p} onClick={() => setPreset(p)} style={{ padding: '6px 14px', borderRadius: 6, border: `1px solid ${preset === p ? '#3fb950' : '#21262d'}`, background: preset === p ? 'rgba(63,185,80,0.1)' : 'rgba(255,255,255,0.03)', color: preset === p ? '#3fb950' : '#8b949e', cursor: 'pointer', fontSize: 12, fontWeight: 600, textTransform: 'capitalize' }}>
                {p}
              </button>
            ))}
            <label style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: '#8b949e', cursor: 'pointer' }}>
              <input type="checkbox" checked={gpu} onChange={(e) => setGpu(e.target.checked)} style={{ accentColor: '#58a6ff' }} />
              GPU preferred
            </label>
          </div>
          <button onClick={onConnect} disabled={connecting} style={{ padding: '10px 20px', borderRadius: 8, border: 'none', background: connecting ? '#1f6feb80' : '#1f6feb', color: '#fff', fontWeight: 700, fontSize: 14, cursor: connecting ? 'wait' : 'pointer', display: 'flex', alignItems: 'center', gap: 8, width: 'fit-content' }}>
            {connecting ? <Loader2 size={16} className="spin" /> : <Play size={16} />}
            {connecting ? 'Connecting...' : 'Connect'}
          </button>
        </>
      )}

      {/* Connected state */}
      {sessionId && (
        <>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '10px 14px', background: 'rgba(255,255,255,0.03)', borderRadius: 8, border: '1px solid rgba(255,255,255,0.07)' }}>
            <span style={{ width: 8, height: 8, borderRadius: '50%', background: STATUS_COLOR[tunnelStatus], flexShrink: 0 }} />
            <span style={{ fontSize: 12, fontWeight: 600, color: '#c9d1d9' }}>Session {sessionId.slice(0, 12)}...</span>
            <span style={{ fontSize: 11, color: '#6e7681', marginLeft: 'auto' }}>{sessionState}</span>
            {rttMs > 0 && <span style={{ fontSize: 11, color: '#3fb950' }}>{rttMs}ms</span>}
            <button onClick={onDisconnect} style={{ padding: '4px 12px', borderRadius: 6, border: '1px solid rgba(248,81,73,0.4)', background: 'rgba(248,81,73,0.1)', color: '#f85149', cursor: 'pointer', fontSize: 11, fontWeight: 600 }}>
              <Square size={12} />
            </button>
          </div>

          <div style={{ position: 'relative', width: '100%', aspectRatio: '16/9', background: '#000', borderRadius: 8, overflow: 'hidden', border: '1px solid rgba(255,255,255,0.08)' }}>
            <video
              ref={videoRef}
              autoPlay
              playsInline
              muted
              style={{ width: '100%', height: '100%', objectFit: 'contain', display: 'block' }}
            />
            {tunnelStatus !== 'connected' && (
              <div style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 10, color: '#6e7681' }}>
                {tunnelStatus === 'connecting' || tunnelStatus === 'reconnecting' ? <Loader2 size={24} className="spin" color="#58a6ff" /> : <Monitor size={24} />}
                <span style={{ fontSize: 12 }}>{tunnelStatus}</span>
              </div>
            )}
          </div>
        </>
      )}

      {/* Log */}
      <div style={{ fontFamily: 'monospace', fontSize: 11, lineHeight: 1.7, color: '#6e7681', maxHeight: 100, overflowY: 'auto', background: 'rgba(0,0,0,0.4)', borderRadius: 6, padding: 8, border: '1px solid rgba(255,255,255,0.05)' }}>
        {log.length === 0 ? <span style={{ color: '#484f58' }}>No events yet</span> : log.slice(-20).map((l, i) => <div key={i} style={{ color: l.includes('error') || l.includes('Error') ? '#f85149' : '#6e7681' }}>{l}</div>)}
      </div>
    </div>
  );
}

function SSHPanel({ sessionId, tunnelStatus, sendSSH }: { sessionId: string | null; tunnelStatus: TunnelStatus; sendSSH: (d: ArrayBuffer) => void }) {
  const [output, setOutput] = useState<string[]>(['FreeCompute SSH Terminal\r\n$ ']);
  const [input, setInput] = useState('');
  const termRef = useRef<HTMLDivElement>(null);
  const enc = useMemo(() => new TextEncoder(), []);
  const mountedRef = useRef(true);
  const timersRef = useRef<Set<ReturnType<typeof setTimeout>>>(new Set());

  useEffect(() => () => {
    mountedRef.current = false;
    for (const t of timersRef.current) clearTimeout(t);
    timersRef.current.clear();
  }, []);

  const submit = useCallback(() => {
    if (!input.trim()) return;
    const cmd = input.trim();
    setOutput((p) => [...p, `${cmd}\r\n`]);
    sendSSH(enc.encode(cmd + '\n').buffer as ArrayBuffer);
    setInput('');
    if (!sessionId || tunnelStatus !== 'connected') {
      // Track the timer so we can cancel it on unmount and avoid a
      // setState-after-unmount warning when the panel closes mid-echo.
      const t = setTimeout(() => {
        timersRef.current.delete(t);
        if (mountedRef.current) setOutput((p) => [...p, `$ `]);
      }, 200);
      timersRef.current.add(t);
    }
  }, [input, sendSSH, enc, sessionId, tunnelStatus]);

  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column', padding: 14, gap: 10 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 11, color: '#6e7681' }}>
        <TerminalIcon size={13} />
        SSH over WebSocket tunnel
        {!sessionId && <span style={{ color: '#f85149', marginLeft: 'auto' }}>Start a connection first</span>}
        {sessionId && <span style={{ color: STATUS_COLOR[tunnelStatus], marginLeft: 'auto' }}>{tunnelStatus}</span>}
      </div>
      <div
        ref={termRef}
        style={{ flex: 1, background: '#010409', borderRadius: 8, padding: 12, fontFamily: 'ui-monospace, monospace', fontSize: 12, lineHeight: 1.6, color: '#c9d1d9', overflowY: 'auto', minHeight: 200, border: '1px solid rgba(255,255,255,0.07)' }}
      >
        {output.map((l, i) => <span key={i}>{l}</span>)}
      </div>
      <div style={{ display: 'flex', gap: 8 }}>
        <span style={{ fontSize: 12, color: '#3fb950', alignSelf: 'center', fontFamily: 'monospace' }}>$</span>
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') submit(); }}
          placeholder="command..."
          disabled={!sessionId}
          style={{ flex: 1, padding: '7px 10px', background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(255,255,255,0.08)', borderRadius: 6, color: '#c9d1d9', fontSize: 12, fontFamily: 'monospace', outline: 'none' }}
        />
        <button onClick={submit} disabled={!sessionId} style={{ padding: '7px 14px', borderRadius: 6, background: '#1f6feb', border: 'none', color: '#fff', fontSize: 12, cursor: 'pointer' }}>Run</button>
      </div>
    </div>
  );
}

function TailscalePanel({ sessionId }: { sessionId: string | null }) {
  const [ip, setIp] = useState('');
  const [copied, setCopied] = useState(false);
  const mountedRef = useRef(true);
  const copyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => () => {
    mountedRef.current = false;
    if (copyTimerRef.current) {
      clearTimeout(copyTimerRef.current);
      copyTimerRef.current = null;
    }
  }, []);

  const copy = useCallback(() => {
    if (!ip) return;
    navigator.clipboard.writeText(ip).then(() => {
      if (!mountedRef.current) return;
      setCopied(true);
      if (copyTimerRef.current) clearTimeout(copyTimerRef.current);
      copyTimerRef.current = setTimeout(() => {
        if (mountedRef.current) setCopied(false);
      }, 2000);
    }).catch(() => { /* clipboard perms denied — silently ignore */ });
  }, [ip]);

  useEffect(() => {
    if (!sessionId) return;
    const gw = getGatewayUrl();
    const token = getTokens()?.accessToken;
    if (!token) return;
    let cancelled = false;
    const controller = new AbortController();
    // /tailscale/user is now authenticated and always returns the CALLER's
    // own entry — never a list. userId is inferred from the JWT server-side.
    fetch(`${gw}/tailscale/user`, {
      method: 'GET',
      headers: {
        'Accept': 'application/json',
        'Authorization': `Bearer ${token}`,
      },
      signal: controller.signal,
    })
      .then((r) => r.ok ? r.json() : null)
      .then((d) => { if (!cancelled && d?.tailscaleIp) setIp(d.tailscaleIp); })
      .catch(() => {});
    return () => { cancelled = true; controller.abort(); };
  }, [sessionId]);

  return (
    <div style={{ flex: 1, padding: 14, display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <Radio size={14} color="#58a6ff" />
        <span style={{ fontSize: 13, fontWeight: 700, color: '#e6edf3' }}>Tailscale Mesh</span>
      </div>
      <p style={{ margin: 0, fontSize: 12, color: '#6e7681', lineHeight: 1.65 }}>
        FreeCompute uses Tailscale to create peer-to-peer tunnels between your browser and the host VM.
        The assigned Tailscale IP allows direct connections for TCP, UDP, SSH, and file transfers — bypassing the relay for lower latency.
      </p>
      {sessionId && ip ? (
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '10px 14px', background: 'rgba(88,166,255,0.06)', border: '1px solid rgba(88,166,255,0.2)', borderRadius: 8 }}>
          <Wifi size={14} color="#58a6ff" />
          <span style={{ fontSize: 13, fontFamily: 'monospace', color: '#58a6ff', flex: 1 }}>{ip}</span>
          <button onClick={copy} style={{ background: 'none', border: 'none', cursor: 'pointer', color: copied ? '#3fb950' : '#6e7681', padding: 4 }}>
            {copied ? <Check size={14} /> : <Copy size={14} />}
          </button>
        </div>
      ) : (
        <div style={{ fontSize: 12, color: '#484f58' }}>
          {sessionId ? 'No Tailscale IP assigned yet. The host agent allocates one on connection.' : 'Start a connection to get your Tailscale mesh IP.'}
        </div>
      )}
      <div style={{ fontSize: 12, color: '#6e7681', lineHeight: 1.65, padding: '10px 14px', background: 'rgba(255,255,255,0.03)', borderRadius: 8, border: '1px solid rgba(255,255,255,0.06)' }}>
        <strong style={{ color: '#c9d1d9' }}>Direct TCP/UDP connection:</strong>
        <br />
        Once the mesh IP is assigned, your session traffic travels peer-to-peer. No relay. No extra hop.
        Average latency reduction: 30–60ms vs proxied connection.
      </div>
    </div>
  );
}

export default function RemoteDesktopApp() {
  const [tab, setTab] = useState<Tab>('stream');
  const [mode, setMode] = useState<Mode>('desktop');
  const [preset, setPreset] = useState<Preset>('safe');
  const [gpu, setGpu] = useState(false);
  const [connecting, setConnecting] = useState(false);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [sessionState, setSessionState] = useState('disconnected');
  const [signalingUrl, setSignalingUrl] = useState<string | null>(null);
  const [log, setLog] = useState<string[]>([]);
  const videoRef = useRef<HTMLVideoElement>(null);

  const addLog = useCallback((msg: string) => {
    const ts = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    setLog((prev) => [...prev.slice(-50), `[${ts}] ${msg}`]);
  }, []);

  const tunnelOpts = useMemo(() => {
    if (!sessionId || !signalingUrl) return null;
    const gw = getGatewayUrl().replace(/^http/, 'ws');
    return { url: `${gw}${signalingUrl}`, sessionId, onStatusChange: (s: TunnelStatus) => addLog(`tunnel: ${s}`) };
  }, [sessionId, signalingUrl, addLog]);

  const tunnel = useTunnelConnection(tunnelOpts);
  const onFrame = tunnel.onFrame;

  // Depend on sessionId so this effect re-runs when the <video> element
  // mounts (the video is conditionally rendered on sessionId inside
  // StreamPanel). Without this dep the effect fires once at parent mount when
  // videoRef.current is still null, early-returns, and never re-runs — so the
  // video pipeline was silently never initialised even after connecting.
  useEffect(() => {
    if (!sessionId) return;
    const video = videoRef.current;
    if (!video || typeof MediaSource === 'undefined') return;

    const mediaSource = new MediaSource();
    const objectUrl = URL.createObjectURL(mediaSource);
    video.src = objectUrl;

    let sourceBuffer: SourceBuffer | null = null;
    const pendingChunks: ArrayBuffer[] = [];
    let disposed = false;

    const flush = () => {
      if (disposed || !sourceBuffer || sourceBuffer.updating) return;
      const next = pendingChunks.shift();
      if (!next) return;
      try {
        sourceBuffer.appendBuffer(next);
      } catch {
        // ignore append errors (buffer full, decode issue, etc.)
      }
    };

    const initSourceBuffer = () => {
      if (disposed) return;
      const mimeCandidates = [
        'video/mp4; codecs="avc1.42E01E"',
        'video/mp4; codecs="hev1.1.6.L93.B0"',
        'video/webm; codecs="vp8"',
        'video/webm; codecs="vp9"',
      ];
      for (const mime of mimeCandidates) {
        if (MediaSource.isTypeSupported(mime)) {
          try {
            sourceBuffer = mediaSource.addSourceBuffer(mime);
            sourceBuffer.mode = 'sequence';
            sourceBuffer.addEventListener('updateend', flush);
            break;
          } catch {
            sourceBuffer = null;
          }
        }
      }
      if (!disposed) flush();
    };

    mediaSource.addEventListener('sourceopen', initSourceBuffer, { once: true });

    // Depend on tunnel.onFrame (a stable useCallback), NOT the whole tunnel
    // object which re-references every render — otherwise this effect would
    // tear down + rebuild MediaSource on every parent re-render and video
    // would never actually play.
    const unsub = onFrame(CH_VIDEO, (payload) => {
      if (disposed) return;
      pendingChunks.push(payload);
      // Cap pending chunk backlog to prevent memory growth if decoder stalls
      if (pendingChunks.length > 60) pendingChunks.splice(0, pendingChunks.length - 60);
      flush();
    });

    video.play().catch(() => { /* autoplay may be blocked until user gesture */ });

    return () => {
      disposed = true;
      unsub();
      try {
        if (sourceBuffer && mediaSource.readyState === 'open') {
          sourceBuffer.removeEventListener('updateend', flush);
          mediaSource.removeSourceBuffer(sourceBuffer);
        }
      } catch { /* ignore */ }
      try {
        if (mediaSource.readyState === 'open') mediaSource.endOfStream();
      } catch { /* ignore */ }
      URL.revokeObjectURL(objectUrl);
      pendingChunks.length = 0;
      if (video) video.src = '';
    };
  }, [onFrame, sessionId]);

  const handleConnect = useCallback(async () => {
    setConnecting(true);
    setSessionState('connecting');
    addLog(`Initiating ${mode} / ${preset} session${gpu ? ' (GPU)' : ''}...`);
    try {
      const gw = getGatewayUrl();
      // Send the JWT so the gateway attributes the session to the current
      // user instead of stamping "anon-<nanos>". Anonymous sessions can't be
      // found by /sessions polling (server filters by userID) and quota /
      // storage accounting never touches the caller's account.
      const token = getTokens()?.accessToken;
      const headers: Record<string, string> = { 'Content-Type': 'application/json' };
      if (token) headers['Authorization'] = `Bearer ${token}`;
      const resp = await fetch(`${gw}/sessions`, {
        method: 'POST',
        headers,
        body: JSON.stringify({ type: mode, mode, streamPreset: preset, gpuPreferred: gpu, resourceClass: 'standard' }),
      });
      if (!resp.ok) {
        const errText = await resp.text().catch(() => `HTTP ${resp.status}`);
        throw new Error(`session create failed (${resp.status}): ${errText.slice(0, 200)}`);
      }
      const data = await resp.json();
      const id = data.session?.id ?? data.sessionId;
      const sigUrl = data.signalingUrl ?? (id ? `/signal/${id}` : null);
      if (id) {
        setSessionId(id);
        setSignalingUrl(sigUrl);
        setSessionState('connected');
        addLog(`Session ${id.slice(0, 12)} established`);
      } else {
        throw new Error(data.error ?? 'no session id returned');
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      addLog(`error: ${msg}`);
      setSessionState('error');
    } finally {
      setConnecting(false);
    }
  }, [mode, preset, gpu, addLog]);

  const handleDisconnect = useCallback(async () => {
    addLog('Disconnecting...');
    tunnel.close();
    if (sessionId) {
      const gw = getGatewayUrl();
      const token = getTokens()?.accessToken;
      const headers: Record<string, string> = { 'Content-Type': 'application/json' };
      if (token) headers['Authorization'] = `Bearer ${token}`;
      await fetch(`${gw}/sessions/${sessionId}`, {
        method: 'DELETE',
        headers,
        body: JSON.stringify({ reason: 'user-requested' }),
      }).catch(() => {});
    }
    setSessionId(null);
    setSignalingUrl(null);
    setSessionState('disconnected');
  }, [tunnel, sessionId, addLog]);

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', background: '#0a0f1e', color: '#c9d1d9', fontFamily: 'system-ui, sans-serif', overflow: 'hidden' }}>
      {/* Header */}
      <div style={{ padding: '12px 16px', background: 'rgba(22,27,34,0.9)', borderBottom: '1px solid rgba(48,54,61,0.5)', display: 'flex', alignItems: 'center', gap: 10, flexShrink: 0 }}>
        <Monitor size={17} color="#58a6ff" />
        <span style={{ fontSize: 14, fontWeight: 700, color: '#e6edf3' }}>Remote Desktop</span>
        {tunnel.rttMs > 0 && (
          <span style={{ marginLeft: 'auto', fontSize: 11, color: tunnel.rttMs < 50 ? '#3fb950' : tunnel.rttMs < 120 ? '#d29922' : '#f85149', display: 'flex', alignItems: 'center', gap: 4 }}>
            <RefreshCw size={10} />
            {tunnel.rttMs}ms RTT
          </span>
        )}
      </div>

      {/* Tabs */}
      <div style={{ display: 'flex', gap: 2, padding: '6px 10px', borderBottom: '1px solid rgba(48,54,61,0.4)', background: 'rgba(13,17,23,0.6)', flexShrink: 0 }}>
        <Tab2 label="Stream" active={tab === 'stream'} onClick={() => setTab('stream')} />
        <Tab2 label="SSH" active={tab === 'ssh'} onClick={() => setTab('ssh')} />
        <Tab2 label="Tailscale" active={tab === 'tailscale'} onClick={() => setTab('tailscale')} />
      </div>

      {tab === 'stream' && (
        <StreamPanel
          mode={mode} setMode={setMode}
          preset={preset} setPreset={setPreset}
          gpu={gpu} setGpu={setGpu}
          sessionId={sessionId} sessionState={sessionState}
          connecting={connecting} rttMs={tunnel.rttMs} tunnelStatus={tunnel.status}
          onConnect={handleConnect} onDisconnect={handleDisconnect}
          videoRef={videoRef} log={log}
        />
      )}
      {tab === 'ssh' && (
        <SSHPanel sessionId={sessionId} tunnelStatus={tunnel.status} sendSSH={tunnel.sendSSH} />
      )}
      {tab === 'tailscale' && (
        <TailscalePanel sessionId={sessionId} />
      )}
    </div>
  );
}
