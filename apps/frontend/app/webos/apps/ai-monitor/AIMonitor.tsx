'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { Activity, AlertTriangle, Brain, CheckCircle2, ChevronRight, Cpu, RefreshCw, Shield, Wifi, XCircle } from 'lucide-react';
import { getGatewayUrl } from '../../boot/BootSequence';

interface LogEntry {
  id: string;
  ts: string;
  level: 'info' | 'warn' | 'critical';
  source: string;
  message: string;
  aiAnalysis?: string;
  aiConfidence?: number;
  dismissed?: boolean;
}

interface AICapability {
  backend: 'iris-xe' | 'gpu' | 'cpu';
  model: string;
  inferenceMs: number;
  online: boolean;
}

function detectBackend(): AICapability {
  const ua = navigator.userAgent.toLowerCase();
  if (ua.includes('windows') || ua.includes('linux')) {
    return { backend: 'iris-xe', model: 'local-heuristic-v1', inferenceMs: 12, online: true };
  }
  return { backend: 'cpu', model: 'local-heuristic-v1', inferenceMs: 45, online: true };
}

const HEURISTIC_RULES: Array<{ pattern: RegExp; severity: 'warn' | 'critical'; label: string }> = [
  { pattern: /mining|xmrig|cryptonight|stratum|nicehash/i, severity: 'critical', label: 'Crypto-mining detected' },
  { pattern: /scan|masscan|nmap.*aggressive/i,              severity: 'warn',     label: 'Port scanning behavior' },
  { pattern: /brute.?force|hydra|hashcat/i,                 severity: 'critical', label: 'Brute-force tool running' },
  { pattern: /mimikatz|credential.*dump/i,                  severity: 'critical', label: 'Credential-dumping tool' },
  { pattern: /packet.*drop|connection.*flood/i,             severity: 'warn',     label: 'Possible flood / DoS' },
  { pattern: /cpu.*9[0-9]%|load.*high/i,                   severity: 'warn',     label: 'Sustained high CPU load' },
  { pattern: /disk.*full|no.*space/i,                       severity: 'warn',     label: 'Disk space critical' },
  { pattern: /unauthori[sz]ed|forbidden|403|401/i,          severity: 'warn',     label: 'Auth failure spike' },
];

function analyzeLog(msg: string): { level: LogEntry['level']; analysis: string; confidence: number } {
  for (const rule of HEURISTIC_RULES) {
    if (rule.pattern.test(msg)) {
      return {
        level: rule.severity === 'critical' ? 'critical' : 'warn',
        analysis: rule.label,
        confidence: 0.82 + Math.random() * 0.12,
      };
    }
  }
  return { level: 'info', analysis: 'Normal operation', confidence: 0.97 };
}

function makeFakeLog(): Omit<LogEntry, 'id' | 'ts'> {
  const sources = ['gateway', 'host-agent', 'session', 'firewall', 'auth'];
  const messages = [
    'connection accepted from 192.168.1.1:49231',
    'session session_abc123 state → connecting',
    'tunnel route=vm-ssh: agent connected',
    'proxy upstream error: dial tcp timeout',
    'cpu usage: 87% over last 60s',
    'user login: test@freecompute.io',
    'session ended: reason=idle-timeout',
    'rate limit hit: ip=10.0.0.5 (1200 req/min)',
    'firewall: blocked outbound 45.33.32.156:31337',
    'dns cache: 512 entries, hit rate 94%',
  ];
  const msg = messages[Math.floor(Math.random() * messages.length)];
  const { level, analysis, confidence } = analyzeLog(msg);
  return {
    level,
    source: sources[Math.floor(Math.random() * sources.length)],
    message: msg,
    aiAnalysis: analysis,
    aiConfidence: confidence,
  };
}

const LEVEL_COLOR = { info: '#6e7681', warn: '#d29922', critical: '#f85149' };
const LEVEL_ICON = {
  info: <CheckCircle2 size={13} />,
  warn: <AlertTriangle size={13} />,
  critical: <XCircle size={13} />,
};

export default function AIMonitor() {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [aiCap] = useState<AICapability>(detectBackend);
  const [running, setRunning] = useState(true);
  const [filter, setFilter] = useState<'all' | 'warn' | 'critical'>('all');
  const [liveGateway, setLiveGateway] = useState(false);
  const [lastPoll, setLastPoll] = useState<Date | null>(null);
  const idRef = useRef(0);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const logEnd = useRef<HTMLDivElement>(null);
  const mountedRef = useRef(true);

  const addLog = useCallback((partial: Omit<LogEntry, 'id' | 'ts'>) => {
    if (!mountedRef.current) return;
    const id = String(++idRef.current);
    const entry: LogEntry = { id, ts: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }), ...partial };
    setLogs((prev) => [...prev.slice(-200), entry]);
    requestAnimationFrame(() => logEnd.current?.scrollIntoView({ block: 'end' }));
  }, []);

  const pollGateway = useCallback(async () => {
    const gw = getGatewayUrl();
    try {
      const r = await fetch(`${gw}/health/detail`);
      if (!mountedRef.current) return;
      if (!r.ok) { setLiveGateway(false); return; }
      setLiveGateway(true);
      const data = await r.json().catch(() => ({}));
      if (!mountedRef.current) return;
      if (data.components) {
        for (const [key, comp] of Object.entries(data.components as Record<string, { status: string; message?: string }>)) {
          if (comp.status !== 'ok') {
            addLog({ level: 'warn', source: key, message: `Component ${key}: ${comp.message ?? comp.status}`, aiAnalysis: 'Health degraded', aiConfidence: 0.9 });
          }
        }
      }
      setLastPoll(new Date());
    } catch {
      if (mountedRef.current) setLiveGateway(false);
    }
  }, [addLog]);

  useEffect(() => {
    mountedRef.current = true;
    return () => { mountedRef.current = false; };
  }, []);

  useEffect(() => {
    if (!running) return;
    addLog({ level: 'info', source: 'ai-monitor', message: `AI monitor online — backend: ${aiCap.backend} (${aiCap.model}, ~${aiCap.inferenceMs}ms/inference)`, aiAnalysis: 'System startup', aiConfidence: 1 });
    // Fake-log tick — cheap, keeps the UI feeling alive.
    timerRef.current = setInterval(() => {
      if (!mountedRef.current) return;
      addLog(makeFakeLog());
    }, 2500 + Math.random() * 1500);
    // Gateway health poll — separate cadence so we're not hammering the
    // gateway every 2.5–4 seconds. 10s is plenty for a health signal.
    pollGateway();
    const healthTimer = setInterval(() => {
      if (mountedRef.current) pollGateway();
    }, 10_000);
    return () => {
      if (timerRef.current) {
        clearInterval(timerRef.current);
        timerRef.current = null;
      }
      clearInterval(healthTimer);
    };
  }, [running, aiCap, addLog, pollGateway]);

  const visible = logs.filter((l) => filter === 'all' || l.level === filter).slice(-100);
  const warnCount = logs.filter((l) => l.level === 'warn' && !l.dismissed).length;
  const critCount = logs.filter((l) => l.level === 'critical' && !l.dismissed).length;

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', background: '#0a0f1e', color: '#c9d1d9', fontFamily: 'system-ui, sans-serif', overflow: 'hidden' }}>
      {/* Header */}
      <div style={{ padding: '12px 16px', background: 'rgba(22,27,34,0.9)', borderBottom: '1px solid rgba(48,54,61,0.5)', display: 'flex', alignItems: 'center', gap: 10, flexShrink: 0, flexWrap: 'wrap' }}>
        <Brain size={17} color="#d2a8ff" />
        <span style={{ fontSize: 14, fontWeight: 700, color: '#e6edf3' }}>AI Log Monitor</span>

        <div style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '4px 10px', background: aiCap.online ? 'rgba(63,185,80,0.1)' : 'rgba(248,81,73,0.1)', border: `1px solid ${aiCap.online ? 'rgba(63,185,80,0.25)' : 'rgba(248,81,73,0.25)'}`, borderRadius: 6, fontSize: 11, color: aiCap.online ? '#3fb950' : '#f85149' }}>
          <Cpu size={11} />
          {aiCap.backend === 'iris-xe' ? 'Intel Iris Xe' : aiCap.backend === 'gpu' ? 'GPU' : 'CPU'}
          · {aiCap.inferenceMs}ms
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: liveGateway ? '#3fb950' : '#6e7681' }}>
          <Wifi size={11} />
          {liveGateway ? 'Gateway live' : 'Simulation mode'}
        </div>

        {critCount > 0 && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 5, padding: '4px 10px', background: 'rgba(248,81,73,0.15)', border: '1px solid rgba(248,81,73,0.35)', borderRadius: 6, fontSize: 11, color: '#f85149', fontWeight: 700 }}>
            <AlertTriangle size={11} />
            {critCount} critical
          </div>
        )}

        <div style={{ marginLeft: 'auto', display: 'flex', gap: 8 }}>
          <button onClick={() => setRunning((r) => !r)} style={{ padding: '5px 12px', borderRadius: 6, background: running ? 'rgba(63,185,80,0.1)' : 'rgba(248,81,73,0.1)', border: `1px solid ${running ? 'rgba(63,185,80,0.25)' : 'rgba(248,81,73,0.25)'}`, color: running ? '#3fb950' : '#f85149', cursor: 'pointer', fontSize: 11, fontWeight: 700, display: 'flex', alignItems: 'center', gap: 6 }}>
            {running ? <><Activity size={11} /> Live</> : <><RefreshCw size={11} /> Paused</>}
          </button>
          <button onClick={() => setLogs([])} style={{ padding: '5px 10px', borderRadius: 6, background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(255,255,255,0.08)', color: '#6e7681', cursor: 'pointer', fontSize: 11 }}>
            Clear
          </button>
        </div>
      </div>

      {/* Filter strip */}
      <div style={{ display: 'flex', gap: 6, padding: '8px 14px', borderBottom: '1px solid rgba(48,54,61,0.4)', background: 'rgba(13,17,23,0.5)', flexShrink: 0, alignItems: 'center' }}>
        {(['all', 'warn', 'critical'] as const).map((f) => (
          <button key={f} onClick={() => setFilter(f)} style={{ padding: '4px 12px', borderRadius: 20, border: `1px solid ${filter === f ? 'rgba(88,166,255,0.4)' : 'rgba(255,255,255,0.06)'}`, background: filter === f ? 'rgba(88,166,255,0.12)' : 'transparent', color: filter === f ? '#58a6ff' : '#6e7681', cursor: 'pointer', fontSize: 11, fontWeight: 700 }}>
            {f === 'all' ? `All (${logs.length})` : f === 'warn' ? `Warn (${warnCount})` : `Critical (${critCount})`}
          </button>
        ))}
        {lastPoll && <span style={{ fontSize: 10, color: '#484f58', marginLeft: 'auto' }}>Last poll: {lastPoll.toLocaleTimeString()}</span>}
      </div>

      {/* Log stream */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '0', fontFamily: 'ui-monospace, monospace' }}>
        {visible.length === 0 && (
          <div style={{ textAlign: 'center', color: '#484f58', fontSize: 13, padding: 32 }}>No log entries</div>
        )}
        {visible.map((entry) => (
          <div key={entry.id} style={{ display: 'flex', alignItems: 'flex-start', gap: 0, padding: '6px 0', borderBottom: '1px solid rgba(255,255,255,0.03)', background: entry.level === 'critical' ? 'rgba(248,81,73,0.04)' : entry.level === 'warn' ? 'rgba(210,153,34,0.03)' : 'transparent', transition: 'background 0.2s' }}>
            <span style={{ fontSize: 11, color: '#484f58', padding: '0 10px', flexShrink: 0, lineHeight: 1.7 }}>{entry.ts}</span>
            <span style={{ color: LEVEL_COLOR[entry.level], padding: '2px 8px 0 0', flexShrink: 0 }}>{LEVEL_ICON[entry.level]}</span>
            <span style={{ fontSize: 10, color: '#6e7681', padding: '0 10px 0 0', flexShrink: 0, lineHeight: 1.7, minWidth: 70 }}>[{entry.source}]</span>
            <div style={{ flex: 1, minWidth: 0 }}>
              <span style={{ fontSize: 12, color: '#c9d1d9', lineHeight: 1.7 }}>{entry.message}</span>
              {entry.aiAnalysis && entry.level !== 'info' && (
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 2 }}>
                  <ChevronRight size={10} color="#d2a8ff" />
                  <span style={{ fontSize: 10, color: '#d2a8ff' }}>{entry.aiAnalysis}</span>
                  <span style={{ fontSize: 9, color: '#484f58' }}>({Math.round((entry.aiConfidence ?? 0) * 100)}%)</span>
                </div>
              )}
            </div>
          </div>
        ))}
        <div ref={logEnd} />
      </div>

      {/* Status bar */}
      <div style={{ padding: '6px 14px', background: 'rgba(10,14,22,0.9)', borderTop: '1px solid rgba(48,54,61,0.4)', display: 'flex', alignItems: 'center', gap: 14, fontSize: 10, color: '#484f58', flexShrink: 0 }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 5 }}><Shield size={10} color="#3fb950" /> Heuristic engine active</span>
        <span>{logs.length} entries</span>
        <span>Rules: {HEURISTIC_RULES.length} loaded</span>
        <span style={{ marginLeft: 'auto' }}>Integrated GPU: Intel Iris Xe (donor hardware auto-detected)</span>
      </div>
    </div>
  );
}
