'use client';

import { useCallback, useEffect, useState } from 'react';
import { apiFetch } from '../../boot/BootSequence';

// Swallow expected auth failures (403/401) without noise; log unexpected errors.
function logUnlessForbidden(e: unknown) {
  const msg = String((e as Error)?.message || e || '');
  if (msg.includes('forbidden') || msg.includes('unauthorized') || msg.includes('401') || msg.includes('403')) {
    return;
  }
  console.error(e);
}

interface DashboardData {
  totalThreats: number;
  pausedVMs: number;
  flaggedVMs: number;
  activeThreats: number;
}

interface UserInfo {
  id: string;
  email: string;
  displayName: string;
  storageUsed: number;
  storageQuota: number;
  tailscaleIp?: string;
  createdAt: string;
}

interface ThreatEvent {
  id: string;
  vmId: string;
  userId: string;
  type: string;
  level: string;
  description: string;
  evidence: any;
  screenshot?: string;
  createdAt: string;
  resolved: boolean;
  action?: string;
}

interface Settings {
  gatewayAddr: string;
  cdnHostname: string;
  edgeHostname: string;
  autoDetectDomain: string;
  maxUsers: number;
  threatDetection: boolean;
  autoPauseOnThreat: boolean;
  requireAiReview: boolean;
  maxConcurrentSessions: number;
  sessionTimeoutMinutes: number;
}

type Tab = 'dashboard' | 'users' | 'threats' | 'settings' | 'domain';

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

export default function AdminApp() {
  const [tab, setTab] = useState<Tab>('dashboard');
  const [dashboard, setDashboard] = useState<DashboardData | null>(null);
  const [users, setUsers] = useState<UserInfo[]>([]);
  const [threats, setThreats] = useState<ThreatEvent[]>([]);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [domain, setDomain] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  const fetchDashboard = useCallback(async () => {
    setLoading(true);
    try {
      const d = await apiFetch('/admin/dashboard');
      setDashboard(d);
    } catch (e) { logUnlessForbidden(e); }
    setLoading(false);
  }, []);

  const fetchUsers = useCallback(async () => {
    try {
      const d = await apiFetch('/admin/users');
      setUsers(d.users || []);
    } catch (e) { logUnlessForbidden(e); }
  }, []);

  const fetchThreats = useCallback(async () => {
    try {
      const d = await apiFetch('/admin/threats');
      setThreats(d.threats || []);
    } catch (e) { logUnlessForbidden(e); }
  }, []);

  const fetchSettings = useCallback(async () => {
    try {
      const s = await apiFetch('/admin/settings');
      setSettings(s);
    } catch (e) { logUnlessForbidden(e); }
  }, []);

  const fetchDomain = useCallback(async () => {
    try {
      const d = await apiFetch('/admin/auto-detect');
      setDomain(d);
    } catch (e) { logUnlessForbidden(e); }
  }, []);

  useEffect(() => { fetchDashboard(); }, []);

  const handleReview = useCallback(async (threatId: string, action: string, resolved: boolean) => {
    try {
      await apiFetch('/admin/threats/review', {
        method: 'POST',
        body: JSON.stringify({ threatId, action, resolved }),
      });
      fetchThreats();
    } catch (e) { logUnlessForbidden(e); }
  }, [fetchThreats]);

  const handleDeleteUser = useCallback(async (userId: string) => {
    try {
      await apiFetch(`/admin/users/delete?userId=${userId}`, { method: 'DELETE' });
      fetchUsers();
    } catch (e) { logUnlessForbidden(e); }
  }, [fetchUsers]);

  const renderTab = () => {
    switch (tab) {
      case 'dashboard':
        return (
          <div>
            <h3 style={{ fontSize: 16, color: '#fff', marginBottom: 16 }}>System Overview</h3>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: 12 }}>
              {[
                { label: 'Active Threats', value: dashboard?.activeThreats ?? 0, color: '#f44' },
                { label: 'Total Threats', value: dashboard?.totalThreats ?? 0, color: '#d29922' },
                { label: 'Paused VMs', value: dashboard?.pausedVMs ?? 0, color: '#f44' },
                { label: 'Flagged VMs', value: dashboard?.flaggedVMs ?? 0, color: '#d29922' },
              ].map((s) => (
                <div key={s.label} style={{ background: '#161b22', borderRadius: 8, padding: 16 }}>
                  <div style={{ fontSize: 11, color: '#888' }}>{s.label}</div>
                  <div style={{ fontSize: 28, fontWeight: 700, color: s.color }}>{s.value}</div>
                </div>
              ))}
            </div>
            <button onClick={fetchDashboard} style={{ marginTop: 12, padding: '6px 16px', background: '#238636', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12 }}>
              {loading ? '...' : 'Refresh'}
            </button>
          </div>
        );

      case 'users':
        return (
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
              <h3 style={{ fontSize: 16, color: '#fff' }}>Users ({users.length})</h3>
              <button onClick={fetchUsers} style={{ padding: '4px 12px', background: '#238636', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12 }}>Refresh</button>
            </div>
            {users.map((u) => (
              <div key={u.id} style={{ display: 'flex', alignItems: 'center', padding: '8px 12px', background: '#161b22', borderRadius: 6, marginBottom: 4 }}>
                <div style={{ flex: 1 }}>
                  <div style={{ fontSize: 13, color: '#ccc' }}>{u.email} <span style={{ color: '#888' }}>({u.displayName})</span></div>
                  <div style={{ fontSize: 11, color: '#555' }}>
                    {u.tailscaleIp && <span>IP: {u.tailscaleIp} | </span>}
                    Storage: {formatBytes(u.storageUsed)} / {formatBytes(u.storageQuota)}
                  </div>
                </div>
                {u.email !== 'admin' && (
                  <button onClick={() => handleDeleteUser(u.id)} style={{ padding: '4px 10px', background: '#da3633', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 11 }}>Delete</button>
                )}
              </div>
            ))}
          </div>
        );

      case 'threats':
        return (
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
              <h3 style={{ fontSize: 16, color: '#fff' }}>Threat Detection ({threats.length})</h3>
              <button onClick={fetchThreats} style={{ padding: '4px 12px', background: '#238636', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12 }}>Refresh</button>
            </div>
            {threats.map((t) => (
              <div key={t.id} style={{ padding: '10px 12px', background: '#161b22', borderRadius: 6, marginBottom: 6, borderLeft: `3px solid ${t.level === 'critical' ? '#f44' : t.level === 'high' ? '#d29922' : '#58a6ff'}` }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                  <span style={{ fontSize: 11, padding: '2px 6px', borderRadius: 4, background: t.level === 'critical' ? '#3d0000' : '#3d2e00', color: t.level === 'critical' ? '#f44' : '#d29922', textTransform: 'uppercase' }}>{t.level}</span>
                  <span style={{ fontSize: 12, color: '#58a6ff' }}>{t.type}</span>
                  <span style={{ fontSize: 11, color: '#888' }}>VM: {t.vmId}</span>
                  <span style={{ fontSize: 11, color: '#555' }}>{new Date(t.createdAt).toLocaleString()}</span>
                </div>
                <div style={{ fontSize: 12, color: '#ccc', marginBottom: t.resolved ? 0 : 8 }}>{t.description}</div>
                {!t.resolved && (
                  <div style={{ display: 'flex', gap: 4 }}>
                    <button onClick={() => handleReview(t.id, 'false-positive', true)} style={{ padding: '4px 10px', background: '#238636', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 11 }}>False Positive</button>
                    <button onClick={() => handleReview(t.id, 'confirmed-malware', true)} style={{ padding: '4px 10px', background: '#da3633', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 11 }}>Confirm + Quarantine</button>
                  </div>
                )}
                {t.resolved && <span style={{ fontSize: 11, color: '#238636' }}>Resolved: {t.action}</span>}
              </div>
            ))}
            {threats.length === 0 && <div style={{ color: '#555', textAlign: 'center', padding: 20 }}>No threats detected</div>}
          </div>
        );

      case 'settings':
        return (
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
              <h3 style={{ fontSize: 16, color: '#fff' }}>System Settings</h3>
              <button onClick={fetchSettings} style={{ padding: '4px 12px', background: '#238636', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12 }}>Refresh</button>
            </div>
            {settings && (
              <div style={{ background: '#161b22', borderRadius: 8, padding: 16 }}>
                {[
                  ['Auto-Detect Domain', settings.autoDetectDomain || 'Not detected'],
                  ['Max Users', settings.maxUsers],
                  ['Threat Detection', settings.threatDetection ? 'Enabled' : 'Disabled'],
                  ['Auto-Pause on Threat', settings.autoPauseOnThreat ? 'Enabled' : 'Disabled'],
                  ['Require AI Review', settings.requireAiReview ? 'Enabled' : 'Disabled'],
                  ['Max Sessions', settings.maxConcurrentSessions],
                  ['Session Timeout', `${settings.sessionTimeoutMinutes} min`],
                  ['Default Storage', formatBytes(10737418240)],
                ].map(([label, value]) => (
                  <div key={label as string} style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: '1px solid #21262d', fontSize: 13 }}>
                    <span style={{ color: '#888' }}>{label}</span>
                    <span style={{ color: '#58a6ff' }}>{String(value)}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
        );

      case 'domain':
        return (
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
              <h3 style={{ fontSize: 16, color: '#fff' }}>Domain Detection</h3>
              <button onClick={fetchDomain} style={{ padding: '4px 12px', background: '#238636', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12 }}>Refresh</button>
            </div>
            {domain && (
              <div style={{ background: '#161b22', borderRadius: 8, padding: 16 }}>
                {[
                  ['Auto-Detected Domain', domain.domain || 'Not detected'],
                  ['Request Host', domain.host],
                  ['Remote Address', domain.remoteAddr],
                  ['Auto-Detected', domain.autoDetected ? 'Yes' : 'No (using localhost)'],
                ].map(([label, value]) => (
                  <div key={label as string} style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: '1px solid #21262d', fontSize: 13 }}>
                    <span style={{ color: '#888' }}>{label}</span>
                    <span style={{ color: '#58a6ff' }}>{String(value)}</span>
                  </div>
                ))}
              </div>
            )}
            <button onClick={fetchDomain} style={{ marginTop: 12, padding: '6px 16px', background: '#1f6feb', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12 }}>Detect Now</button>
          </div>
        );
    }
  };

  return (
    <div style={{ display: 'flex', height: '100%', background: '#0d1117', color: '#ccc', fontFamily: 'system-ui, sans-serif' }}>
      <div style={{ width: 180, background: '#161b22', borderRight: '1px solid #21262d', padding: 8 }}>
        <div style={{ fontSize: 14, fontWeight: 600, color: '#fff', padding: '8px 12px', marginBottom: 8 }}>Admin Panel</div>
        {(['dashboard', 'users', 'threats', 'settings', 'domain'] as Tab[]).map((t) => (
          <button key={t} onClick={() => { setTab(t); if (t === 'users') fetchUsers(); if (t === 'threats') fetchThreats(); if (t === 'settings') fetchSettings(); if (t === 'domain') fetchDomain(); }}
            style={{ display: 'block', width: '100%', padding: '8px 12px', background: tab === t ? '#1f6feb' : 'none', border: 'none', color: tab === t ? '#fff' : '#888', borderRadius: 4, cursor: 'pointer', fontSize: 12, textAlign: 'left', marginBottom: 2 }}>
            {t.charAt(0).toUpperCase() + t.slice(1)}
          </button>
        ))}
      </div>
      <div style={{ flex: 1, padding: 16, overflow: 'auto' }}>
        {renderTab()}
      </div>
    </div>
  );
}
