'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { Monitor, Play, Square, RefreshCw, Wifi, Globe, CheckCircle2, XCircle, Loader2 } from 'lucide-react';
import { createSession, createWebRTCSession } from '../webos/system/api/websocket';
import { defaultConnectionConfig } from '../webos/system/types';
import { getGatewayUrl } from '../webos/boot/BootSequence';

type SessionMode = 'desktop' | 'development' | 'gaming' | 'remote-support';
type ResourceClass = 'basic' | 'standard' | 'gaming' | 'workstation';
type StreamPreset = 'safe' | 'fast';
type StreamTransport = 'webrtc' | 'websocket-fallback' | 'quic' | 'webtransport';

type SessionState = 'created' | 'queued' | 'provisioning' | 'connecting' | 'active' | 'reconnecting' | 'ended' | 'expired' | 'failed';

interface AppSession {
  id: string;
  state: SessionState;
  mode: string;
  resourceClass: string;
  streamPreset: string;
  transport: StreamTransport;
  signalingUrl?: string;
  connectionToken?: string;
  turnServers?: string[];
  estimatedReady?: number;
  error?: string;
}

type StreamStatus = 'idle' | 'connecting' | 'connected' | 'failed' | 'closed';

function time() {
  return new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

const stateColor: Record<SessionState, string> = {
  queued: '#a96800',
  provisioning: '#246bfe',
  connecting: '#246bfe',
  active: '#198754',
  reconnecting: '#d29922',
  ended: '#607065',
  expired: '#607065',
  failed: '#b42318',
};

const streamStatusColor: Record<StreamStatus, string> = {
  idle: '#8b949e',
  connecting: '#246bfe',
  connected: '#198754',
  failed: '#b42318',
  closed: '#607065',
};

function buildSignalUrl(signalingUrl: string): string {
  const gateway = getGatewayUrl();
  const wsBase = gateway.replace(/^http/, 'ws');
  return `${wsBase}${signalingUrl}`;
}

const FALLBACK_ICE_SERVERS: RTCIceServer[] = [
  { urls: 'stun:stun.l.google.com:19302' },
];

interface StreamViewerProps {
  session: AppSession;
  turnServers?: string[];
  onLog: (msg: string) => void;
  onClose: (id: string) => void;
}

function buildIceServers(turnServers?: string[]): RTCIceServer[] {
  const servers: RTCIceServer[] = [];
  if (turnServers && turnServers.length > 0) {
    for (const url of turnServers) {
      servers.push({ urls: url });
    }
  }
  for (const fallback of FALLBACK_ICE_SERVERS) {
    if (!servers.some((s) => s.urls === fallback.urls)) {
      servers.push(fallback);
    }
  }
  return servers;
}

function StreamViewer({ session, turnServers, onLog, onClose }: StreamViewerProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const pcRef = useRef<RTCPeerConnection | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const dataChannelRef = useRef<RTCDataChannel | null>(null);
  const cleanupRef = useRef<(() => void) | null>(null);

  const [status, setStatus] = useState<StreamStatus>('idle');
  const [streamError, setStreamError] = useState('');
  const [remoteStream, setRemoteStream] = useState<MediaStream | null>(null);

  const sendInput = useCallback((event: any) => {
    const dc = dataChannelRef.current;
    if (dc && dc.readyState === 'open') {
      try {
        dc.send(JSON.stringify(event));
        onLog(`[input] ${event.type} sent`);
      } catch (e: any) {
        onLog(`[input] send failed: ${e.message}`);
      }
    } else {
      onLog(`[input] data channel not open, dropped ${event.type}`);
    }
  }, [onLog]);

  const handleMouse = useCallback((e: React.MouseEvent) => {
    const rect = videoRef.current?.getBoundingClientRect();
    sendInput({
      type: 'mouse',
      kind: e.type,
      x: rect ? Math.round(e.clientX - rect.left) : e.clientX,
      y: rect ? Math.round(e.clientY - rect.top) : e.clientY,
      buttons: e.buttons,
    });
  }, [sendInput]);

  const handleKey = useCallback((e: React.KeyboardEvent) => {
    sendInput({
      type: 'keyboard',
      kind: e.type,
      key: e.key,
      code: e.code,
      ctrlKey: e.ctrlKey,
      shiftKey: e.shiftKey,
      altKey: e.altKey,
      metaKey: e.metaKey,
    });
  }, [sendInput]);

  useEffect(() => {
    if (!session.signalingUrl) return;

    let cancelled = false;
    setStatus('connecting');
    setStreamError('');
    onLog(`Stream: opening WebSocket → ${buildSignalUrl(session.signalingUrl)}`);

    const ws = new WebSocket(buildSignalUrl(session.signalingUrl));
    wsRef.current = ws;

    ws.onopen = () => {
      if (cancelled) return;
      onLog(`Stream: WebSocket open, creating PeerConnection`);

      const pc = new RTCPeerConnection({ iceServers: buildIceServers(turnServers) });
      pcRef.current = pc;

      pc.addTransceiver('video', { direction: 'recvonly' });
      pc.addTransceiver('audio', { direction: 'recvonly' });

      pc.onicecandidate = (ev) => {
        if (ev.candidate && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'ice-candidate', payload: ev.candidate }));
        }
      };

      pc.onconnectionstatechange = () => {
        if (cancelled) return;
        onLog(`Stream: pc state → ${pc.connectionState}`);
        if (pc.connectionState === 'connected') setStatus('connected');
        else if (pc.connectionState === 'failed' || pc.connectionState === 'disconnected') {
          setStatus('failed');
          setStreamError(`PeerConnection ${pc.connectionState}`);
        }
      };

      pc.ontrack = (ev) => {
        if (cancelled) return;
        onLog(`Stream: received ${ev.track.kind} track`);
    let stream = ev.streams[0];
    if (!stream) {
        stream = new MediaStream();
        stream.addTrack(ev.track);
    }
    setRemoteStream(stream);
    if (videoRef.current) {
        videoRef.current.srcObject = stream;
        videoRef.current.play().catch((err) => onLog(`Stream: autoplay blocked: ${err.message}`));
    }
      };

      const dc = pc.createDataChannel('input', { ordered: true });
      dataChannelRef.current = dc;
      dc.onopen = () => onLog(`Stream: input data channel open`);
      dc.onclose = () => onLog(`Stream: input data channel closed`);
      dc.onerror = (err) => onLog(`Stream: input data channel error`);

      pc.createOffer()
        .then((offer) => pc.setLocalDescription(offer))
        .then(() => {
          if (ws.readyState === WebSocket.OPEN && pc.localDescription) {
            ws.send(JSON.stringify({ type: 'offer', payload: pc.localDescription }));
            onLog(`Stream: offer sent (${pc.localDescription.type})`);
          }
        })
        .catch((err) => {
          onLog(`Stream: createOffer failed: ${err.message}`);
          setStatus('failed');
          setStreamError(err.message);
        });
    };

    ws.onmessage = (msg) => {
      if (cancelled) return;
      let parsed: any;
      try {
        parsed = JSON.parse(msg.data);
      } catch (e: any) {
        onLog(`Stream: invalid WS message`);
        return;
      }

      const pc = pcRef.current;
      switch (parsed.type) {
        case 'answer':
          if (pc) {
            onLog(`Stream: answer received`);
            pc.setRemoteDescription(new RTCSessionDescription(parsed.payload))
              .catch((err) => {
                onLog(`Stream: setRemoteDescription failed: ${err.message}`);
                setStatus('failed');
                setStreamError(err.message);
              });
          }
          break;
        case 'ice-candidate':
          if (pc && parsed.payload) {
            pc.addIceCandidate(new RTCIceCandidate(parsed.payload)).catch((err) =>
              onLog(`Stream: addIceCandidate failed: ${err.message}`)
            );
          }
          break;
        default:
          onLog(`Stream: unknown message type "${parsed.type}"`);
      }
    };

    ws.onerror = (ev) => {
      if (cancelled) return;
      onLog(`Stream: WebSocket error`);
      setStatus('failed');
      setStreamError('WebSocket signaling error');
    };

    ws.onclose = () => {
      if (cancelled) return;
      onLog(`Stream: WebSocket closed`);
      if (status !== 'failed') setStatus('closed');
    };

    cleanupRef.current = () => {
      cancelled = true;
      try { ws.close(); } catch {}
      try {
        const dc = dataChannelRef.current;
        if (dc) dc.close();
      } catch {}
      try {
        const pc = pcRef.current;
        if (pc) pc.close();
      } catch {}
      wsRef.current = null;
      pcRef.current = null;
      dataChannelRef.current = null;
      setRemoteStream(null);
      if (videoRef.current) videoRef.current.srcObject = null;
    };

    return () => {
      cleanupRef.current?.();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session.id, session.signalingUrl]);

  const close = useCallback(() => {
    cleanupRef.current?.();
    setStatus('closed');
    onLog(`Stream: viewer closed for ${session.id.slice(0, 8)}...`);
    onClose(session.id);
  }, [onClose, onLog, session.id]);

  return (
    <div style={{ background: '#161b22', border: '1px solid #30363d', borderRadius: 12, padding: 20 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
        <div style={{ fontSize: 13, fontWeight: 600, color: '#8b949e', textTransform: 'uppercase', letterSpacing: 1, display: 'flex', alignItems: 'center', gap: 8 }}>
          <Monitor size={14} color="#58a6ff" />
          Stream Viewer
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontSize: 11, padding: '2px 8px', borderRadius: 4, background: '#30363d', color: streamStatusColor[status], textTransform: 'capitalize', display: 'flex', alignItems: 'center', gap: 6 }}>
            <span style={{ width: 7, height: 7, borderRadius: '50%', background: streamStatusColor[status], display: 'inline-block' }} />
            {status}
          </span>
          <button type="button" onClick={close}
            style={{ padding: '4px 10px', background: 'rgba(248,81,73,0.1)', border: '1px solid rgba(248,81,73,0.4)', color: '#f85149', borderRadius: 4, cursor: 'pointer', fontSize: 11 }}>
            Close
          </button>
        </div>
      </div>

      <div style={{ position: 'relative', width: '100%', aspectRatio: '16 / 9', background: '#000', borderRadius: 8, overflow: 'hidden', border: '1px solid #30363d' }}>
        <video
          ref={videoRef}
          autoPlay
          playsInline
          muted
          onMouseDown={handleMouse}
          onMouseUp={handleMouse}
          onMouseMove={handleMouse}
          onContextMenu={(e) => e.preventDefault()}
          onKeyDown={handleKey}
          tabIndex={0}
          style={{ width: '100%', height: '100%', objectFit: 'contain', display: remoteStream ? 'block' : 'none', cursor: 'crosshair', outline: 'none' }}
        />
        {status !== 'connected' && (
          <div style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 10, color: '#8b949e' }}>
            {status === 'connecting' && <Loader2 size={28} className="spin" color="#58a6ff" />}
            {status === 'failed' && <XCircle size={28} color="#f85149" />}
            {status === 'closed' && <Square size={28} color="#607065" />}
            <div style={{ fontSize: 13 }}>
              {status === 'connecting' && 'Establishing WebRTC connection...'}
              {status === 'failed' && (streamError || 'Connection failed')}
              {status === 'closed' && 'Stream closed'}
              {status === 'idle' && 'Waiting to start...'}
            </div>
          </div>
        )}
      </div>

      {remoteStream && (
        <div style={{ marginTop: 10, fontSize: 11, color: '#8b949e' }}>
          Click the video and type / move the mouse to send input (logged below). Session: <code style={{ color: '#58a6ff' }}>{session.id.slice(0, 12)}...</code>
        </div>
      )}
    </div>
  );
}

export default function ConnectPage() {
  const [mode, setMode] = useState<SessionMode>('desktop');
  const [resourceClass, setResourceClass] = useState<ResourceClass>('standard');
  const [preset, setPreset] = useState<StreamPreset>('safe');
  const [transport, setTransport] = useState<StreamTransport>('webrtc');
  const [gpuPreferred, setGpuPreferred] = useState(false);
  const [gpuRequired, setGpuRequired] = useState(false);
  const [connecting, setConnecting] = useState(false);
  const [sessions, setSessions] = useState<AppSession[]>([]);
  const [logs, setLogs] = useState<string[]>([]);
  const [error, setError] = useState('');
  const [viewId, setViewId] = useState<string | null>(null);


  const addLog = useCallback((msg: string) => {
    setLogs((prev) => [...prev, `[${time()}] ${msg}`]);
  }, []);

  const startSession = useCallback(async () => {
    setConnecting(true);
    setError('');
    addLog(`Starting ${mode} session (${resourceClass}, ${preset}, ${transport})...`);

    const cfg = {
      ...defaultConnectionConfig(),
      sessionMode: mode,
      resourceClass,
      preset,
      transport,
      gpuPreferred,
      gpuRequired,
    };

    try {
      let data: any = null;
      try {
        data = await createSession(cfg);
        addLog(`Session created via /sessions/: ${data.session?.id || data.id || 'unknown'}`);
      } catch (e: any) {
        addLog(`/sessions/ failed: ${e.message}. Trying /webrtc/ fallback...`);
        try {
          data = await createWebRTCSession(cfg);
          addLog(`WebRTC session created: ${data.sessionId}`);
        } catch (e2: any) {
          throw new Error(`Both /sessions/ and /webrtc/ failed: ${e2.message}`);
        }
      }

      const session: AppSession = {
        id: data.session?.id || data.sessionId || `sess_${Date.now().toString(36)}`,
        state: data.session?.state || data.state || (data.signalingUrl ? 'connecting' : 'queued'),
        mode,
        resourceClass,
        streamPreset: preset,
        transport,
        signalingUrl: data.signalingUrl,
        connectionToken: data.connectionToken,
        estimatedReady: data.estimatedReady,
      };

      setSessions((prev) => [session, ...prev]);
      addLog(`Session ${session.id.slice(0, 8)}... created — waiting for stream`);
    } catch (e: any) {
      const msg = e.message || 'Failed to start session';
      setError(msg);
      addLog(`ERROR: ${msg}`);
    } finally {
      setConnecting(false);
    }
  }, [mode, resourceClass, preset, transport, gpuPreferred, gpuRequired, addLog]);

  useEffect(() => {
    const interval = setInterval(async () => {
      try {
        const resp = await fetch(`${getGatewayUrl()}/sessions`, {
          headers: { 'Accept': 'application/json' },
        });
        if (!resp.ok) return;
        const data = await resp.json();
        if (!Array.isArray(data.sessions)) return;

        setSessions((prev) => {
          const map = new Map(prev.map((s) => [s.id, s]));
          const next: AppSession[] = [];
          for (const s of data.sessions) {
            const id = s.id || s.sessionId;
            if (!id) continue;
            const existing = map.get(id);
            next.push({
              id,
              state: (s.state as SessionState) || existing?.state || 'queued',
              mode: s.mode || s.type || existing?.mode || 'desktop',
              resourceClass: s.resourceClass || existing?.resourceClass || 'standard',
              streamPreset: s.streamPreset || existing?.streamPreset || 'safe',
              transport: existing?.transport || 'webrtc',
              signalingUrl: s.signalingUrl || existing?.signalingUrl || `/signal/${id}`,
              connectionToken: s.connectionToken || existing?.connectionToken,
              estimatedReady: s.estimatedReady || existing?.estimatedReady,
            });
          }
          if (next.length === 0) return prev;
          if (!viewId) {
            const auto = next.find((s) => s.signalingUrl && (s.state === 'active' || s.state === 'connecting' || s.state === 'created'));
            if (auto?.signalingUrl) {
              setViewId(auto.id);
              addLog(`Auto-discovered session ${auto.id.slice(0, 8)}... → attaching viewer`);
            }
          }
          return next;
        });
      } catch {
        // ignore poll errors
      }
    }, 3000);
    return () => clearInterval(interval);
  }, [getGatewayUrl, addLog, viewId]);


  const disconnect = useCallback(async (id: string) => {
    addLog(`Disconnecting session ${id.slice(0, 8)}...`);
    try {
      await fetch(`${getGatewayUrl()}/sessions/${id}`, { method: 'DELETE' });
    } catch {
      try {
        await fetch(`${getGatewayUrl()}/webrtc/${id}`, { method: 'DELETE' });
      } catch {
        // ignore backend errors, still clean up locally
      }
    }
    setSessions((prev) => prev.filter((s) => s.id !== id));
    if (viewId === id) setViewId(null);
  }, [addLog, viewId]);

  const closeViewer = useCallback((id: string) => {
    setViewId((cur) => (cur === id ? null : cur));
  }, []);

  const modes: { value: SessionMode; label: string; desc: string }[] = [
    { value: 'desktop', label: 'Desktop', desc: 'Remote desktop access' },
    { value: 'development', label: 'Development', desc: 'IDE and terminal' },
    { value: 'gaming', label: 'Gaming', desc: 'Low-latency game stream' },
    { value: 'remote-support', label: 'Remote Support', desc: 'Guarded access with approval' },
  ];

  const resources: { value: ResourceClass; label: string; desc: string }[] = [
    { value: 'basic', label: 'Basic', desc: '1 vCPU, 2GB RAM' },
    { value: 'standard', label: 'Standard', desc: '2 vCPU, 4GB RAM' },
    { value: 'gaming', label: 'Gaming', desc: '4 vCPU, 8GB + GPU' },
    { value: 'workstation', label: 'Workstation', desc: '8 vCPU, 16GB + GPU' },
  ];

  const viewSession = sessions.find((s) => s.id === viewId) || null;

  return (
    <div style={{ minHeight: '100vh', background: '#0d1117', color: '#c9d1d9', fontFamily: 'system-ui, -apple-system, sans-serif' }}>
      <div style={{ maxWidth: 1200, margin: '0 auto', padding: 24 }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 24, flexWrap: 'wrap', gap: 12 }}>
          <div>
            <h1 style={{ margin: 0, fontSize: 28, fontWeight: 700, color: '#e6edf3', display: 'flex', alignItems: 'center', gap: 10 }}>
              <Monitor size={28} color="#58a6ff" />
              Connection Space
            </h1>
            <p style={{ margin: '4px 0 0', color: '#8b949e', fontSize: 14 }}>Start and manage VM connections through FreeCompute</p>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '6px 14px', background: '#161b22', border: '1px solid #30363d', borderRadius: 8 }}>
            <Wifi size={16} color="#238636" />
            <span style={{ fontSize: 13, color: '#238636', fontWeight: 600 }}>{getGatewayUrl().replace(/^https?:\/\//, '')}</span>
          </div>
        </div>

        {error && (
          <div style={{ marginBottom: 16, padding: 12, background: 'rgba(248,81,73,0.1)', border: '1px solid rgba(248,81,73,0.4)', borderRadius: 8, color: '#f85149', fontSize: 13, display: 'flex', alignItems: 'center', gap: 8 }}>
            <XCircle size={16} />
            {error}
            <button onClick={() => setError('')} style={{ background: 'none', border: 'none', color: '#f85149', cursor: 'pointer', marginLeft: 'auto' }}><XCircle size={14} /></button>
          </div>
        )}

        <div style={{ display: 'grid', gridTemplateColumns: '320px 1fr', gap: 16 }}>
          <div style={{ background: '#161b22', border: '1px solid #30363d', borderRadius: 12, padding: 20, height: 'fit-content' }}>
            <div style={{ fontSize: 13, fontWeight: 600, color: '#8b949e', marginBottom: 12, textTransform: 'uppercase', letterSpacing: 1 }}>New Connection</div>

            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 13, color: '#c9d1d9', marginBottom: 8 }}>Mode</div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                {modes.map((m) => (
                  <button key={m.value} type="button" onClick={() => setMode(m.value)}
                    style={{
                      padding: 10, borderRadius: 6, border: '1px solid', cursor: 'pointer', textAlign: 'left', transition: 'all 0.15s',
                      background: mode === m.value ? '#1f6feb22' : '#0d1117',
                      borderColor: mode === m.value ? '#1f6feb' : '#30363d',
                    }}>
                    <div style={{ fontSize: 13, fontWeight: 600, color: mode === m.value ? '#58a6ff' : '#c9d1d9' }}>{m.label}</div>
                    <div style={{ fontSize: 11, color: '#8b949e', marginTop: 2 }}>{m.desc}</div>
                  </button>
                ))}
              </div>
            </div>

            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 13, color: '#c9d1d9', marginBottom: 8 }}>Resource Class</div>
              <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                {resources.map((r) => (
                  <button key={r.value} type="button" onClick={() => setResourceClass(r.value)}
                    style={{
                      padding: '6px 12px', borderRadius: 6, border: '1px solid', cursor: 'pointer', fontSize: 12, transition: 'all 0.15s',
                      background: resourceClass === r.value ? '#1f6feb' : '#0d1117',
                      borderColor: resourceClass === r.value ? '#1f6feb' : '#30363d',
                      color: resourceClass === r.value ? '#fff' : '#8b949e',
                    }}>
                    {r.label}
                  </button>
                ))}
              </div>
            </div>

            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 13, color: '#c9d1d9', marginBottom: 8 }}>Stream Preset</div>
              <div style={{ display: 'flex', gap: 4 }}>
                {(['safe', 'fast'] as StreamPreset[]).map((p) => (
                  <button key={p} type="button" onClick={() => setPreset(p)}
                    style={{
                      flex: 1, padding: '8px', borderRadius: 6, border: '1px solid', cursor: 'pointer', fontSize: 12, textTransform: 'capitalize', transition: 'all 0.15s',
                      background: preset === p ? '#23863622' : '#0d1117',
                      borderColor: preset === p ? '#238636' : '#30363d',
                      color: preset === p ? '#3fb950' : '#8b949e',
                    }}>
                    {p}
                  </button>
                ))}
              </div>
            </div>

            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 13, color: '#c9d1d9', marginBottom: 8 }}>Transport</div>
              <select value={transport} onChange={(e) => setTransport(e.target.value as StreamTransport)}
                style={{ width: '100%', padding: 8, background: '#0d1117', border: '1px solid #30363d', borderRadius: 6, color: '#c9d1d9', fontSize: 13 }}>
                <option value="webrtc">WebRTC</option>
                <option value="websocket-fallback">WebSocket</option>
                <option value="quic">QUIC</option>
                <option value="webtransport">WebTransport</option>
              </select>
            </div>

            <div style={{ marginBottom: 20, display: 'flex', flexDirection: 'column', gap: 8 }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer', fontSize: 13, color: '#c9d1d9' }}>
                <input type="checkbox" checked={gpuPreferred} onChange={(e) => setGpuPreferred(e.target.checked)} />
                GPU Preferred
              </label>
              <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer', fontSize: 13, color: '#c9d1d9' }}>
                <input type="checkbox" checked={gpuRequired} onChange={(e) => setGpuRequired(e.target.checked)} />
                GPU Required
              </label>
            </div>

            <button type="button" onClick={startSession} disabled={connecting}
              style={{
                width: '100%', padding: 12, background: connecting ? '#1f6feb66' : '#1f6feb', border: 'none', borderRadius: 8, color: '#fff', fontWeight: 600, cursor: connecting ? 'wait' : 'pointer', fontSize: 14, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
              }}>
              {connecting ? <Loader2 size={18} className="spin" /> : <Play size={18} />}
              {connecting ? 'Connecting...' : 'Connect'}
            </button>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div style={{ background: '#161b22', border: '1px solid #30363d', borderRadius: 12, padding: 20, flex: 1 }}>
              <div style={{ fontSize: 13, fontWeight: 600, color: '#8b949e', marginBottom: 12, textTransform: 'uppercase', letterSpacing: 1 }}>Active Sessions</div>
              {sessions.length === 0 ? (
                <div style={{ color: '#484f58', textAlign: 'center', padding: 40, fontSize: 14 }}>No active sessions. Start a connection to begin.</div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                  {sessions.map((s) => (
                    <div key={s.id} style={{ padding: 14, background: '#0d1117', border: '1px solid #30363d', borderRadius: 8 }}>
                      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          <div style={{ width: 8, height: 8, borderRadius: '50%', background: stateColor[s.state] }} />
                          <span style={{ fontSize: 13, fontWeight: 600, color: '#e6edf3' }}>Session {s.id.slice(0, 12)}...</span>
                        </div>
                        <span style={{ fontSize: 11, padding: '2px 8px', borderRadius: 4, background: '#30363d', color: '#c9d1d9', textTransform: 'capitalize' }}>{s.state}</span>
                      </div>
                      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 6, fontSize: 12, color: '#8b949e', marginBottom: 10 }}>
                        <div>Mode: <span style={{ color: '#c9d1d9' }}>{s.mode}</span></div>
                        <div>Class: <span style={{ color: '#c9d1d9' }}>{s.resourceClass}</span></div>
                        <div>Preset: <span style={{ color: '#c9d1d9' }}>{s.streamPreset}</span></div>
                        <div>Transport: <span style={{ color: '#c9d1d9' }}>{s.transport}</span></div>
                        {s.signalingUrl && <div style={{ gridColumn: '1 / -1' }}>Signal: <code style={{ color: '#58a6ff', fontSize: 11 }}>{s.signalingUrl}</code></div>}
                        {s.estimatedReady && <div style={{ gridColumn: '1 / -1' }}>Est. ready: <span style={{ color: '#c9d1d9' }}>{s.estimatedReady}s</span></div>}
                      </div>
                      <div style={{ display: 'flex', gap: 8 }}>
                        {s.signalingUrl && (s.state === 'connecting' || s.state === 'active') && (
                          viewId !== s.id ? (
                            <button type="button" onClick={() => setViewId(s.id)}
                              style={{ fontSize: 11, color: '#58a6ff', background: 'none', border: '1px solid #1f6feb', borderRadius: 4, cursor: 'pointer', padding: '4px 10px', display: 'flex', alignItems: 'center', gap: 4 }}>
                              <Monitor size={12} /> View Stream
                            </button>
                          ) : (
                            <span style={{ fontSize: 11, color: '#58a6ff', display: 'flex', alignItems: 'center', gap: 4 }}><Wifi size={12} /> Streaming</span>
                          )
                        )}
                        {s.state === 'active' && (
                          <span style={{ fontSize: 11, color: '#3fb950', display: 'flex', alignItems: 'center', gap: 4 }}><CheckCircle2 size={12} /> Stream active</span>
                        )}
                        <button type="button" onClick={() => disconnect(s.id)}
                          style={{ marginLeft: 'auto', padding: '4px 10px', background: 'rgba(248,81,73,0.1)', border: '1px solid rgba(248,81,73,0.4)', color: '#f85149', borderRadius: 4, cursor: 'pointer', fontSize: 11 }}>
                          Disconnect
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {viewSession && viewSession.signalingUrl && (
              <StreamViewer session={viewSession} turnServers={viewSession.turnServers as string[] | undefined} onLog={addLog} onClose={closeViewer} />
            )}

            <div style={{ background: '#0d1117', border: '1px solid #30363d', borderRadius: 12, padding: 16, maxHeight: 300, display: 'flex', flexDirection: 'column' }}>
              <div style={{ fontSize: 13, fontWeight: 600, color: '#8b949e', marginBottom: 10, textTransform: 'uppercase', letterSpacing: 1, display: 'flex', alignItems: 'center', gap: 8 }}>
                <Globe size={14} />
                Connection Log
              </div>
              <div style={{ flex: 1, overflowY: 'auto', fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace', fontSize: 12, lineHeight: 1.6 }}>
                {logs.length === 0 && <span style={{ color: '#484f58' }}>No events yet.</span>}
                {logs.map((l, i) => (
                  <div key={i} style={{ color: l.includes('ERROR') ? '#f85149' : l.includes('→') ? '#3fb950' : l.includes('Stream') ? '#58a6ff' : '#8b949e' }}>{l}</div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
