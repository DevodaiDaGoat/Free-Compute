'use client';

import { useState, useCallback } from 'react';
import { Monitor, Play, Square, Wifi, Loader2 } from 'lucide-react';
import { createSession, createWebRTCSession } from '../../system/api/websocket';
import { defaultConnectionConfig } from '../../system/types';
import { getGatewayUrl } from '../../boot/BootSequence';

type Mode = 'desktop' | 'development' | 'gaming' | 'remote-support';
type Preset = 'safe' | 'fast';

export default function RemoteDesktopApp() {
  const [mode, setMode] = useState<Mode>('desktop');
  const [preset, setPreset] = useState<Preset>('safe');
  const [gpu, setGpu] = useState(false);
  const [connecting, setConnecting] = useState(false);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [sessionState, setSessionState] = useState('disconnected');
  const [log, setLog] = useState<string[]>([]);

  const addLog = useCallback((msg: string) => {
    const ts = new Date().toLocaleTimeString();
    setLog((prev) => [...prev, `[${ts}] ${msg}`]);
  }, []);

  const handleConnect = useCallback(async () => {
    setConnecting(true);
    setSessionState('connecting');
    addLog(`Initiating ${mode} session (preset: ${preset}, gpu: ${gpu})`);
    try {
      const config = defaultConnectionConfig();
      config.sessionMode = mode;
      config.preset = preset;
      config.gpuPreferred = gpu;

      let id: string | null = null;
      let session = await createSession(config);
      if (session?.session?.id) {
        id = session.session.id;
        addLog(`Session established: ${id}`);
      } else {
        addLog('Primary session failed, trying WebRTC fallback...');
        const webrtc = await createWebRTCSession(config);
        if (webrtc?.sessionId) {
          id = webrtc.sessionId;
          addLog(`WebRTC session established: ${id}`);
        }
      }

      if (id) {
        setSessionId(id);
        setSessionState('connected');
      } else {
        addLog('Failed to establish session');
        setSessionState('error');
      }
    } catch (err) {
      addLog(`Error: ${err instanceof Error ? err.message : String(err)}`);
      setSessionState('error');
    } finally {
      setConnecting(false);
    }
  }, [mode, preset, gpu, addLog]);

  const handleDisconnect = useCallback(async () => {
    addLog('Disconnecting...');
    if (sessionId) {
      try {
        await fetch(`${getGatewayUrl()}/sessions/${sessionId}`, { method: 'DELETE' });
      } catch {
        try {
          await fetch(`${getGatewayUrl()}/webrtc/${sessionId}`, { method: 'DELETE' });
        } catch {
          // ignore backend errors, still clean up locally
        }
      }
    }
    setSessionId(null);
    setSessionState('disconnected');
  }, [addLog, sessionId]);

  const modes: { key: Mode; label: string }[] = [
    { key: 'desktop', label: 'Desktop' },
    { key: 'development', label: 'Development' },
    { key: 'gaming', label: 'Gaming' },
    { key: 'remote-support', label: 'Remote Support' },
  ];

  return (
    <div style={{
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      background: '#0d1117',
      color: '#c9d1d9',
      fontFamily: 'system-ui, sans-serif',
      overflow: 'hidden',
    }}>
      <div style={{
        padding: 16,
        background: '#161b22',
        borderBottom: '1px solid #30363d',
        display: 'flex',
        alignItems: 'center',
        gap: 12,
      }}>
        <Monitor size={20} color="#58a6ff" />
        <span style={{ fontSize: 16, fontWeight: 600 }}>Remote Desktop</span>
      </div>

      <div style={{ flex: 1, overflow: 'auto', padding: 16 }}>
        <div style={{ marginBottom: 16 }}>
          <div style={{ fontSize: 12, color: '#8b949e', marginBottom: 8, textTransform: 'uppercase', letterSpacing: 1 }}>Connection Mode</div>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            {modes.map((m) => (
              <button
                key={m.key}
                onClick={() => setMode(m.key)}
                style={{
                  padding: '8px 16px',
                  borderRadius: 6,
                  border: `1px solid ${mode === m.key ? '#58a6ff' : '#30363d'}`,
                  background: mode === m.key ? 'rgba(88,166,255,0.15)' : '#161b22',
                  color: mode === m.key ? '#58a6ff' : '#c9d1d9',
                  cursor: 'pointer',
                  fontSize: 13,
                  fontWeight: mode === m.key ? 600 : 400,
                }}
              >
                {m.label}
              </button>
            ))}
          </div>
        </div>

        <div style={{ marginBottom: 16 }}>
          <div style={{ fontSize: 12, color: '#8b949e', marginBottom: 8, textTransform: 'uppercase', letterSpacing: 1 }}>Preset</div>
          <div style={{ display: 'flex', gap: 8 }}>
            {(['safe', 'fast'] as Preset[]).map((p) => (
              <button
                key={p}
                onClick={() => setPreset(p)}
                style={{
                  padding: '8px 16px',
                  borderRadius: 6,
                  border: `1px solid ${preset === p ? '#58a6ff' : '#30363d'}`,
                  background: preset === p ? 'rgba(88,166,255,0.15)' : '#161b22',
                  color: preset === p ? '#58a6ff' : '#c9d1d9',
                  cursor: 'pointer',
                  fontSize: 13,
                  textTransform: 'capitalize',
                }}
              >
                {p}
              </button>
            ))}
          </div>
        </div>

        <div style={{ marginBottom: 16, display: 'flex', alignItems: 'center', gap: 8 }}>
          <input
            type="checkbox"
            id="gpu-pref"
            checked={gpu}
            onChange={(e) => setGpu(e.target.checked)}
            style={{ accentColor: '#58a6ff' }}
          />
          <label htmlFor="gpu-pref" style={{ fontSize: 13, cursor: 'pointer' }}>GPU Preferred</label>
        </div>

        <div style={{ marginBottom: 16, display: 'flex', gap: 8 }}>
          {!sessionId ? (
            <button
              onClick={handleConnect}
              disabled={connecting}
              style={{
                padding: '10px 24px',
                borderRadius: 6,
                border: 'none',
                background: connecting ? '#23863688' : '#238636',
                color: '#fff',
                cursor: connecting ? 'not-allowed' : 'pointer',
                fontSize: 14,
                fontWeight: 600,
                display: 'flex',
                alignItems: 'center',
                gap: 8,
              }}
            >
              {connecting ? <Loader2 size={16} className="spin" /> : <Play size={16} />}
              {connecting ? 'Connecting...' : 'Connect'}
            </button>
          ) : (
            <button
              onClick={handleDisconnect}
              style={{
                padding: '10px 24px',
                borderRadius: 6,
                border: 'none',
                background: '#da3633',
                color: '#fff',
                cursor: 'pointer',
                fontSize: 14,
                fontWeight: 600,
                display: 'flex',
                alignItems: 'center',
                gap: 8,
              }}
            >
              <Square size={16} />
              Disconnect
            </button>
          )}
        </div>

        {sessionId && (
          <div style={{
            padding: 12,
            background: '#161b22',
            border: '1px solid #30363d',
            borderRadius: 8,
            marginBottom: 16,
            display: 'flex',
            alignItems: 'center',
            gap: 12,
          }}>
            <Wifi size={16} color="#3fb950" />
            <div>
              <div style={{ fontSize: 12, color: '#8b949e' }}>Session</div>
              <div style={{ fontSize: 14, fontWeight: 600, color: '#c9d1d9' }}>{sessionId}</div>
            </div>
            <div style={{ marginLeft: 'auto', fontSize: 12, color: sessionState === 'connected' ? '#3fb950' : '#d29922', textTransform: 'capitalize' }}>
              {sessionState}
            </div>
          </div>
        )}

        <div>
          <div style={{ fontSize: 12, color: '#8b949e', marginBottom: 8, textTransform: 'uppercase', letterSpacing: 1 }}>Log</div>
          <div style={{
            background: '#010409',
            border: '1px solid #30363d',
            borderRadius: 8,
            padding: 12,
            height: 160,
            overflowY: 'auto',
            fontFamily: 'monospace',
            fontSize: 12,
            lineHeight: 1.6,
          }}>
            {log.length === 0 ? (
              <span style={{ color: '#484f58' }}>No log entries yet...</span>
            ) : (
              log.map((entry, i) => (
                <div key={i} style={{ color: '#c9d1d9' }}>{entry}</div>
              ))
            )}
          </div>
        </div>
      </div>

      <style>{`
        @keyframes spin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }
        .spin {
          animation: spin 1s linear infinite;
        }
      `}</style>
    </div>
  );
}
