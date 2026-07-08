'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { apiFetch, getTokens, getUser } from '../../boot/BootSequence';
import type { ConnectionConfig, Preferences } from '../../system/types';
import { defaultConnectionConfig } from '../../system/types';
import ConnectionSettings from './ConnectionSettings';

function mergeConnection(base: ConnectionConfig, partial?: Partial<ConnectionConfig>): ConnectionConfig {
  if (!partial) return base;
  return { ...base, ...partial };
}

export default function SettingsApp() {
  const user = getUser();
  const [tab, setTab] = useState<'account' | 'connection'>('account');
  const [tailIP, setTailIP] = useState('');
  const [allocating, setAllocating] = useState(false);
  const [connConfig, setConnConfig] = useState<ConnectionConfig>(defaultConnectionConfig());
  const [savingPrefs, setSavingPrefs] = useState(false);
  const [prefsError, setPrefsError] = useState('');
  const loaded = useRef(false);

  const allocateIP = useCallback(async () => {
    setAllocating(true);
    try {
      const data = await apiFetch('/auth/tailscale-ip', { method: 'POST' });
      setTailIP(data.tailscaleIp);
    } catch (err) {
      console.error('allocate ip error:', err);
    } finally {
      setAllocating(false);
    }
  }, []);

  useEffect(() => {
    if (user?.tailscaleIp) setTailIP(user.tailscaleIp);
  }, [user]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const data = await apiFetch('/auth/preferences') as Preferences;
        if (cancelled) return;
        setConnConfig((prev) => mergeConnection(prev, data?.connection));
      } catch (err) {
        console.warn('load preferences unavailable:', err);
      } finally {
        if (!cancelled) loaded.current = true;
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const savePreferences = useCallback(async () => {
    setPrefsError('');
    setSavingPrefs(true);
    try {
      await apiFetch('/auth/preferences', { method: 'PUT', body: JSON.stringify({ connection: connConfig }) });
    } catch (err) {
      console.error('save preferences error:', err);
      setPrefsError('Save failed');
    } finally {
      setSavingPrefs(false);
    }
  }, [connConfig]);

  useEffect(() => {
    if (!loaded.current) return;
    const t = setTimeout(() => { void savePreferences(); }, 600);
    return () => clearTimeout(t);
  }, [connConfig, savePreferences]);

  const percent = user ? (user.storageUsed / user.storageQuota) * 100 : 0;

  const tabStyle = (t: string): React.CSSProperties => ({
    padding: '8px 16px', borderRadius: 6, border: 'none', cursor: 'pointer', fontSize: 13,
    background: tab === t ? '#1f6feb' : 'transparent', color: tab === t ? '#fff' : '#8b949e',
    transition: 'all 0.15s',
  });

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', background: '#0d1117', fontFamily: 'system-ui, sans-serif' }}>
      <div style={{ padding: '12px 16px', borderBottom: '1px solid #21262d', display: 'flex', gap: 8, alignItems: 'center' }}>
        <h2 style={{ fontSize: 16, fontWeight: 600, color: '#e6edf3', margin: 0 }}>Settings</h2>
        <div style={{ flex: 1 }} />
        <button type="button" style={tabStyle('account')} onClick={() => setTab('account')}>Account</button>
        <button type="button" style={tabStyle('connection')} onClick={() => setTab('connection')}>Connection</button>
      </div>

      {tab === 'account' && (
        <div style={{ padding: 16, overflow: 'auto', flex: 1 }}>
          <section style={{ marginBottom: 24 }}>
            <h3 style={{ fontSize: 14, color: '#888', marginBottom: 8 }}>Account</h3>
            <div style={{ background: '#161b22', borderRadius: 8, padding: 16 }}>
              <div style={{ marginBottom: 8, fontSize: 13 }}>Email: <span style={{ color: '#58a6ff' }}>{user?.email || 'N/A'}</span></div>
              <div style={{ marginBottom: 8, fontSize: 13 }}>Display Name: <span style={{ color: '#58a6ff' }}>{user?.displayName || 'N/A'}</span></div>
              <div style={{ marginBottom: 8, fontSize: 13 }}>User ID: <span style={{ color: '#888', fontSize: 11 }}>{user?.id || 'N/A'}</span></div>
            </div>
          </section>

          <section style={{ marginBottom: 24 }}>
            <h3 style={{ fontSize: 14, color: '#888', marginBottom: 8 }}>Network</h3>
            <div style={{ background: '#161b22', borderRadius: 8, padding: 16 }}>
              <div style={{ marginBottom: 8, fontSize: 13 }}>
                Tailscale IP: <span style={{ color: tailIP ? '#58a6ff' : '#888' }}>{tailIP || 'Not allocated'}</span>
                {!tailIP && (
                  <button onClick={allocateIP} disabled={allocating}
                    style={{ marginLeft: 12, padding: '4px 12px', background: '#238636', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 11 }}>
                    {allocating ? '...' : 'Allocate IP'}
                  </button>
                )}
              </div>
              <div style={{ fontSize: 13 }}>Proxy Mode: <span style={{ color: '#888' }}>relay (auto)</span></div>
            </div>
          </section>

          <section style={{ marginBottom: 24 }}>
            <h3 style={{ fontSize: 14, color: '#888', marginBottom: 8 }}>Storage</h3>
            <div style={{ background: '#161b22', borderRadius: 8, padding: 16 }}>
              <div style={{ height: 8, background: '#21262d', borderRadius: 4, marginBottom: 8 }}>
                <div style={{ width: `${Math.min(percent, 100)}%`, height: '100%', background: percent > 80 ? '#d29922' : '#238636', borderRadius: 4 }} />
              </div>
              <div style={{ fontSize: 13 }}>
                {formatBytes(user?.storageUsed || 0)} / {formatBytes(user?.storageQuota || 0)} used
              </div>
            </div>
          </section>
        </div>
      )}

      {tab === 'connection' && (
        <div style={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }}>
          <div style={{ padding: '8px 16px', borderBottom: '1px solid #21262d', display: 'flex', alignItems: 'center', gap: 8 }}>
            <button
              type="button"
              onClick={() => void savePreferences()}
              disabled={savingPrefs}
              style={{ padding: '4px 12px', background: '#238636', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 11 }}
            >
              {savingPrefs ? 'Saving…' : 'Save'}
            </button>
            {prefsError && <span style={{ fontSize: 11, color: '#f85149' }}>{prefsError}</span>}
          </div>
          <div style={{ flex: 1, minHeight: 0 }}>
            <ConnectionSettings config={connConfig} onChange={setConnConfig} />
          </div>
        </div>
      )}
    </div>
  );
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}
