'use client';

import { useState } from 'react';
import type { AppWindow, DesktopApp } from '../system/types';
import { Monitor } from 'lucide-react';

interface Props {
  windows: AppWindow[];
  apps: DesktopApp[];
  onOpenApp: (id: string) => void;
  onFocus: (id: string) => void;
}

export default function Taskbar({ windows, apps, onOpenApp, onFocus }: Props) {
  const [showMenu, setShowMenu] = useState(false);
  const now = new Date();
  const timeStr = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

  return (
    <div style={{
      height: 44,
      background: '#0d0d1a',
      borderTop: '1px solid #1e2a4a',
      display: 'flex',
      alignItems: 'center',
      padding: '0 8px',
      position: 'relative',
      zIndex: 9999,
      flexShrink: 0,
      backdropFilter: 'blur(12px)',
    }}>
      <button
        onClick={() => setShowMenu(!showMenu)}
        style={{
          background: showMenu ? '#2a2a5a' : 'rgba(255,255,255,0.03)',
          border: '1px solid ' + (showMenu ? '#3a3a6a' : '#1a1a3e'),
          color: showMenu ? '#18e2ff' : '#ccc',
          padding: '6px 14px',
          borderRadius: 6,
          cursor: 'pointer',
          fontSize: 13,
          fontWeight: 600,
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          transition: 'all 0.15s ease',
        }}
      >
        <Monitor size={16} />
        Start
      </button>

      <div style={{ display: 'flex', gap: 2, marginLeft: 8, flex: 1, overflow: 'hidden' }}>
        {windows.map((win) => (
          <button
            key={win.id}
            onClick={() => onFocus(win.id)}
            style={{
              background: win.focused ? '#2a2a5a' : 'rgba(255,255,255,0.02)',
              border: '1px solid ' + (win.focused ? 'rgba(24,226,255,0.3)' : 'transparent'),
              color: win.focused ? '#18e2ff' : '#888',
              padding: '4px 12px',
              borderRadius: 4,
              cursor: 'pointer',
              fontSize: 12,
              maxWidth: 140,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
              transition: 'all 0.15s ease',
            }}
          >
            {win.title}
          </button>
        ))}
      </div>

      <div style={{ fontSize: 11, color: '#888', padding: '4px 10px', background: 'rgba(0,0,0,0.2)', borderRadius: 4 }}>{timeStr}</div>

      {showMenu && (
        <>
          <div style={{ position: 'fixed', bottom: 44, left: 0, width: 240, background: '#111128', border: '1px solid #2a2a4a', borderRadius: '8px 8px 0 0', padding: 4, zIndex: 10000 }}
            onClick={() => setShowMenu(false)}>
            {apps.map((app) => (
              <button
                key={app.id}
                onClick={() => { onOpenApp(app.id); setShowMenu(false); }}
                style={{ display: 'flex', alignItems: 'center', gap: 10, width: '100%', padding: '8px 12px', background: 'none', border: '1px solid transparent', color: '#ccc', cursor: 'pointer', fontSize: 13, borderRadius: 4, transition: 'all 0.1s ease' }}
              >
                <span style={{ display: 'flex', alignItems: 'center', color: '#18e2ff', width: 20, justifyContent: 'center' }}>{app.icon}</span>
                <span>{app.name}</span>
              </button>
            ))}
          </div>
          <div style={{ position: 'fixed', inset: 0, zIndex: 9999 }} onClick={() => setShowMenu(false)} />
        </>
      )}
    </div>
  );
}
