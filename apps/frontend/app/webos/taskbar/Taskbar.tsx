'use client';

import { useState } from 'react';
import type { AppWindow, DesktopApp } from '../system/types';

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
      borderTop: '1px solid #1a1a3e',
      display: 'flex',
      alignItems: 'center',
      padding: '0 8px',
      zIndex: 9999,
      flexShrink: 0,
    }}>
      <button
        onClick={() => setShowMenu(!showMenu)}
        style={{
          background: showMenu ? '#2a2a5a' : 'none', border: 'none', color: '#ccc',
          padding: '6px 14px', borderRadius: 4, cursor: 'pointer', fontSize: 14,
          fontWeight: 600,
        }}
      >
        Start
      </button>

      <div style={{ display: 'flex', gap: 2, marginLeft: 8, flex: 1, overflow: 'hidden' }}>
        {windows.map((win) => (
          <button
            key={win.id}
            onClick={() => onFocus(win.id)}
            style={{
              background: win.focused ? '#2a2a5a' : 'transparent',
              border: 'none', color: win.focused ? '#18e2ff' : '#888',
              padding: '4px 12px', borderRadius: 4, cursor: 'pointer',
              fontSize: 12, maxWidth: 140, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
            }}
          >
            {win.title}
          </button>
        ))}
      </div>

      <div style={{ fontSize: 11, color: '#888' }}>{timeStr}</div>

      {showMenu && (
        <>
          <div style={{ position: 'fixed', bottom: 44, left: 0, width: 240, background: '#111128', border: '1px solid #2a2a4a', borderRadius: '8px 8px 0 0', padding: 8, zIndex: 10000 }}
            onClick={() => setShowMenu(false)}>
            {apps.map((app) => (
              <button
                key={app.id}
                onClick={() => { onOpenApp(app.id); setShowMenu(false); }}
                style={{ display: 'flex', alignItems: 'center', gap: 10, width: '100%', padding: '8px 12px', background: 'none', border: 'none', color: '#ccc', cursor: 'pointer', fontSize: 13, borderRadius: 4 }}
              >
                <span>{app.icon}</span>
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
