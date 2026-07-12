'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { Activity, Cpu, HardDrive, Monitor, RefreshCw, Server, Users, Wifi, Zap } from 'lucide-react';
import { getGatewayUrl, getTokens } from '../../boot/BootSequence';

interface SessionInfo {
  id: string;
  userId: string;
  type: string;
  mode: string;
  status: string;
  protocol: string;
  resourceClass: string;
  createdAt: string;
}

interface HostInfo {
  id: string;
  name: string;
  region: string;
  status: string;
  totalCpu: number;
  usedCpu: number;
  totalMemoryMb: number;
  usedMemoryMb: number;
  totalDiskMb: number;
  usedDiskMb: number;
  gpuModel: string;
  gpuVramMb: number;
  loadAvg1: number;
  tailscaleIp: string;
  lastHeartbeat: string;
}

interface DashboardData {
  totalThreats: number;
  pausedVMs: number;
  flaggedVMs: number;
  activeThreats: number;
  settings: Record<string, unknown>;
}

function pct(used: number, total: number) {
  if (!total) return 0;
  return Math.min(100, Math.round((used / total) * 100));
}

function fmtMb(mb: number) {
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  return `${mb} MB`;
}

function Bar({ value, warn = 70, crit = 90 }: { value: number; warn?: number; crit?: number }) {
  const color = value >= crit ? '#f85149' : value >= warn ? '#d29922' : '#3fb950';
  return (
    <div style={{ height: 4, borderRadius: 2, background: 'rgba(255,255,255,0.06)', overflow: 'hidden', marginTop: 4 }}>
      <div style={{ height: '100%', width: `${value}%`, background: color, borderRadius: 2, transition: 'width 0.6s ease' }} />
    </div>
  );
}

function StatBox({ label, value, sub, icon, color = '#58a6ff' }: { label: string; value: string; sub?: string; icon: React.ReactNode; color?: string }) {
  return (
    <div style={{ padding: '14px 16px', background: 'rgba(255,255,255,0.03)', borderRadius: 10, border: '1px solid rgba(255,255,255,0.06)' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
        <span style={{ color }}>{icon}</span>
        <span style={{ fontSize: 11, color: '#6e7681', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em' }}>{label}</span>
      </div>
      <div style={{ fontSize: 22, fontWeight: 700, color: '#e6edf3', letterSpacing: '-0.02em' }}>{value}</div>
      {sub && <div style={{ fontSize: 11, color: '#6e7681', marginTop: 3 }}>{sub}</div>}
    </div>
  );
}

export default function HostMonitor() {
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [hosts, setHosts] = useState<HostInfo[]>([]);
  const [dashboard, setDashboard] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [lastRefresh, setLastRefresh] = useState<Date | null>(null);
  const refreshTimer = useRef<ReturnType<typeof setInterval> | null>(null);
  const mountedRef = useRef(true);
  // Track whether the current user has permission for /admin/dashboard so we
  // stop hammering it (Chrome logs a red "Failed to load resource" line for
  // every 4xx even though the app handles it gracefully — noisy for non-admins).
  const adminAllowedRef = useRef<boolean | null>(null);

  const refresh = useCallback(async () => {
    const gw = getGatewayUrl();
    const token = getTokens()?.accessToken;
    const headers: Record<string, string> = { 'Accept': 'application/json' };
    if (token) headers['Authorization'] = `Bearer ${token}`;

    const requests: Promise<Response>[] = [
      fetch(`${gw}/sessions`, { headers }),
      fetch(`${gw}/hosts/`, { headers }),
    ];
    if (adminAllowedRef.current !== false) {
      requests.push(fetch(`${gw}/admin/dashboard`, { headers }));
    }
    const settled = await Promise.allSettled(requests);
    const [sessRes, hostRes, dashRes] = settled;

    if (!mountedRef.current) return;

    if (sessRes.status === 'fulfilled' && sessRes.value.ok) {
      const data = await sessRes.value.json().catch(() => ({}));
      if (mountedRef.current) setSessions(Array.isArray(data.sessions) ? data.sessions : []);
    }
    if (hostRes.status === 'fulfilled' && hostRes.value.ok) {
      const data = await hostRes.value.json().catch(() => ({}));
      if (mountedRef.current) setHosts(Array.isArray(data.hosts) ? data.hosts : []);
    }
    if (dashRes) {
      if (dashRes.status === 'fulfilled') {
        if (dashRes.value.ok) {
          adminAllowedRef.current = true;
          const data = await dashRes.value.json().catch(() => null);
          if (mountedRef.current && data) setDashboard(data);
        } else if (dashRes.value.status === 401 || dashRes.value.status === 403) {
          adminAllowedRef.current = false;
        }
      }
    }
    if (!mountedRef.current) return;
    setLoading(false);
    setLastRefresh(new Date());
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    refresh();
    refreshTimer.current = setInterval(refresh, 8000);
    return () => {
      mountedRef.current = false;
      if (refreshTimer.current) {
        clearInterval(refreshTimer.current);
        refreshTimer.current = null;
      }
    };
  }, [refresh]);

  const activeSessions = sessions.filter((s) => s.status === 'active');
  const onlineHosts = hosts.filter((h) => h.status === 'online');

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', background: '#0a0f1e', color: '#c9d1d9', fontFamily: 'system-ui, sans-serif', overflow: 'hidden' }}>

      {/* Header */}
      <div style={{ padding: '14px 18px', background: 'rgba(22,27,34,0.9)', borderBottom: '1px solid rgba(48,54,61,0.6)', display: 'flex', alignItems: 'center', gap: 10, flexShrink: 0 }}>
        <Activity size={18} color="#58a6ff" />
        <span style={{ fontSize: 15, fontWeight: 700, color: '#e6edf3' }}>Host Monitor</span>
        <span style={{ marginLeft: 'auto', fontSize: 11, color: '#484f58' }}>
          {lastRefresh ? `Updated ${lastRefresh.toLocaleTimeString()}` : 'Loading...'}
        </span>
        <button
          onClick={refresh}
          style={{ padding: '5px 10px', background: 'rgba(88,166,255,0.1)', border: '1px solid rgba(88,166,255,0.2)', borderRadius: 6, color: '#58a6ff', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6, fontSize: 12 }}
        >
          <RefreshCw size={12} />
          Refresh
        </button>
      </div>

      {loading ? (
        <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#6e7681', fontSize: 13 }}>
          Loading host data...
        </div>
      ) : (
        <div style={{ flex: 1, overflowY: 'auto', padding: 16, display: 'flex', flexDirection: 'column', gap: 16 }}>

          {/* Summary stats */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 10 }}>
            <StatBox label="Active Sessions" value={String(activeSessions.length)} sub={`of ${sessions.length} total`} icon={<Monitor size={14} />} color="#58a6ff" />
            <StatBox label="Online Hosts" value={String(onlineHosts.length)} sub={`of ${hosts.length} registered`} icon={<Server size={14} />} color="#3fb950" />
            <StatBox label="Active Threats" value={String(dashboard?.activeThreats ?? 0)} sub={dashboard?.pausedVMs ? `${dashboard.pausedVMs} VMs paused` : 'All clear'} icon={<Zap size={14} />} color={dashboard?.activeThreats ? '#f85149' : '#6e7681'} />
            <StatBox label="Connected Users" value={String(new Set(sessions.map((s) => s.userId)).size)} sub="unique users" icon={<Users size={14} />} color="#d2a8ff" />
          </div>

          {/* Active sessions */}
          {sessions.length > 0 && (
            <div>
              <div style={{ fontSize: 11, color: '#6e7681', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.08em', marginBottom: 10 }}>Active Sessions</div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                {sessions.map((s) => (
                  <div key={s.id} style={{ padding: '10px 14px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.06)', borderRadius: 8, display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
                    <span style={{ width: 7, height: 7, borderRadius: '50%', background: s.status === 'active' ? '#3fb950' : '#6e7681', flexShrink: 0 }} />
                    <span style={{ fontSize: 12, fontWeight: 600, color: '#c9d1d9', fontFamily: 'monospace' }}>{s.id.slice(0, 14)}...</span>
                    <span style={{ fontSize: 11, color: '#6e7681', padding: '2px 8px', background: 'rgba(88,166,255,0.08)', borderRadius: 4, border: '1px solid rgba(88,166,255,0.15)' }}>{s.type || 'desktop'}</span>
                    <span style={{ fontSize: 11, color: '#8b949e' }}>{s.mode}</span>
                    <span style={{ fontSize: 11, color: '#8b949e' }}>{s.protocol}</span>
                    <span style={{ fontSize: 11, color: '#8b949e', marginLeft: 'auto' }}>{s.resourceClass}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {sessions.length === 0 && (
            <div style={{ padding: '24px 0', textAlign: 'center', color: '#484f58', fontSize: 13 }}>No active sessions</div>
          )}

          {/* Hosts */}
          {hosts.length > 0 && (
            <div>
              <div style={{ fontSize: 11, color: '#6e7681', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.08em', marginBottom: 10 }}>Compute Hosts</div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                {hosts.map((h) => {
                  const cpuPct = pct(h.usedCpu, h.totalCpu);
                  const ramPct = pct(h.usedMemoryMb, h.totalMemoryMb);
                  const diskPct = pct(h.usedDiskMb, h.totalDiskMb);
                  return (
                    <div key={h.id} style={{ padding: 14, background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.06)', borderRadius: 10 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 12 }}>
                        <span style={{ width: 8, height: 8, borderRadius: '50%', background: h.status === 'online' ? '#3fb950' : '#6e7681', flexShrink: 0 }} />
                        <span style={{ fontSize: 13, fontWeight: 700, color: '#e6edf3' }}>{h.name || h.id.slice(0, 14)}</span>
                        <span style={{ fontSize: 11, color: '#6e7681' }}>{h.region}</span>
                        {h.tailscaleIp && (
                          <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: '#58a6ff', marginLeft: 'auto' }}>
                            <Wifi size={11} />
                            {h.tailscaleIp}
                          </span>
                        )}
                      </div>
                      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12 }}>
                        <div>
                          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                            <span style={{ fontSize: 11, color: '#6e7681', display: 'flex', alignItems: 'center', gap: 4 }}><Cpu size={11} /> CPU</span>
                            <span style={{ fontSize: 11, fontWeight: 600, color: '#c9d1d9' }}>{cpuPct}%</span>
                          </div>
                          <Bar value={cpuPct} />
                          <span style={{ fontSize: 10, color: '#484f58' }}>{h.usedCpu}/{h.totalCpu} cores</span>
                        </div>
                        <div>
                          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                            <span style={{ fontSize: 11, color: '#6e7681', display: 'flex', alignItems: 'center', gap: 4 }}><Activity size={11} /> RAM</span>
                            <span style={{ fontSize: 11, fontWeight: 600, color: '#c9d1d9' }}>{ramPct}%</span>
                          </div>
                          <Bar value={ramPct} />
                          <span style={{ fontSize: 10, color: '#484f58' }}>{fmtMb(h.usedMemoryMb)} / {fmtMb(h.totalMemoryMb)}</span>
                        </div>
                        <div>
                          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                            <span style={{ fontSize: 11, color: '#6e7681', display: 'flex', alignItems: 'center', gap: 4 }}><HardDrive size={11} /> Disk</span>
                            <span style={{ fontSize: 11, fontWeight: 600, color: '#c9d1d9' }}>{diskPct}%</span>
                          </div>
                          <Bar value={diskPct} warn={80} crit={95} />
                          <span style={{ fontSize: 10, color: '#484f58' }}>{fmtMb(h.usedDiskMb)} / {fmtMb(h.totalDiskMb)}</span>
                        </div>
                      </div>
                      {h.gpuModel && (
                        <div style={{ marginTop: 8, fontSize: 11, color: '#6e7681' }}>
                          GPU: <span style={{ color: '#d2a8ff' }}>{h.gpuModel}</span>
                          {h.gpuVramMb > 0 && ` · ${fmtMb(h.gpuVramMb)} VRAM`}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {hosts.length === 0 && (
            <div style={{ padding: '24px 0', textAlign: 'center', color: '#484f58', fontSize: 13 }}>No hosts registered. Run the Host Agent to contribute compute.</div>
          )}

        </div>
      )}
    </div>
  );
}
