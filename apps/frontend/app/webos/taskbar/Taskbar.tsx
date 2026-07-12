'use client';

import { useEffect, useState } from 'react';
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
  const [timeStr, setTimeStr] = useState(() =>
    new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
  );

  useEffect(() => {
    const tick = () => setTimeStr(new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }));
    tick();
    const id = setInterval(tick, 15_000);
    return () => clearInterval(id);
  }, []);

  return (
    <div style={{
      height: 46,
      background: 'rgba(10,14,22,0.92)',
      borderTop: '1px solid rgba(48,54,61,0.7)',
      display: 'flex',
      alignItems: 'center',
      padding: '0 10px',
      position: 'relative',
      zIndex: 9999,
      flexShrink: 0,
      backdropFilter: 'blur(16px)',
      gap: 6,
    }}>
      {/* Start button */}
      <button
        onClick={() => setShowMenu(!showMenu)}
        style={{
          background: showMenu ? 'rgba(88,166,255,0.2)' : 'rgba(255,255,255,0.04)',
          border: `1px solid ${showMenu ? 'rgba(88,166,255,0.4)' : 'rgba(255,255,255,0.08)'}`,
          color: showMenu ? '#58a6ff' : '#c9d1d9',
          padding: '5px 14px',
          borderRadius: 7,
          cursor: 'pointer',
          fontSize: 13,
          fontWeight: 600,
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          transition: 'all 0.12s',
          flexShrink: 0,
        }}
      >
        <Monitor size={15} />
        Apps
      </button>

      {/* Separator */}
      <div style={{ width: 1, height: 22, background: 'rgba(255,255,255,0.08)', flexShrink: 0 }} />

      {/* Open windows */}
      <div style={{ display: 'flex', gap: 3, flex: 1, overflow: 'hidden' }}>
        {windows.map((win) => (
          <button
            key={win.id}
            onClick={() => onFocus(win.id)}
            style={{
              background: win.focused
                ? 'rgba(88,166,255,0.15)'
                : win.minimized
                ? 'rgba(255,255,255,0.02)'
                : 'rgba(255,255,255,0.04)',
              border: `1px solid ${win.focused ? 'rgba(88,166,255,0.35)' : 'rgba(255,255,255,0.06)'}`,
              color: win.focused ? '#58a6ff' : '#8b949e',
              padding: '4px 12px',
              borderRadius: 6,
              cursor: 'pointer',
              fontSize: 12,
              fontWeight: 500,
              maxWidth: 160,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
              transition: 'all 0.12s',
              display: 'flex',
              alignItems: 'center',
              gap: 5,
            }}
          >
            {win.minimized && (
              <span style={{ width: 5, height: 5, borderRadius: '50%', background: 'currentColor', opacity: 0.5 }} />
            )}
            {win.title}
          </button>
        ))}
      </div>

      {/* Clock */}
      <div style={{
        fontSize: 12,
        color: '#8b949e',
        padding: '4px 10px',
        background: 'rgba(255,255,255,0.04)',
        borderRadius: 6,
        border: '1px solid rgba(255,255,255,0.06)',
        fontWeight: 600,
        flexShrink: 0,
      }}>
        {timeStr}
      </div>

      {/* Start menu popup */}
      {showMenu && (
        <>
          <div
            style={{
              position: 'fixed',
              bottom: 46,
              left: 0,
              width: 260,
              background: 'rgba(13,17,23,0.96)',
              border: '1px solid rgba(48,54,61,0.8)',
              borderRadius: '10px 10px 0 0',
              padding: 6,
              zIndex: 10000,
              backdropFilter: 'blur(16px)',
              boxShadow: '0 -8px 32px rgba(0,0,0,0.5)',
            }}
            onClick={() => setShowMenu(false)}
          >
            <div style={{ padding: '6px 10px 8px', fontSize: 11, color: '#6e7681', textTransform: 'uppercase', letterSpacing: '0.08em', fontWeight: 600 }}>
              Applications
            </div>
            {apps.map((app) => (
              <button
                key={app.id}
                onClick={() => { onOpenApp(app.id); setShowMenu(false); }}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 10,
                  width: '100%',
                  padding: '8px 10px',
                  background: 'none',
                  border: '1px solid transparent',
                  color: '#c9d1d9',
                  cursor: 'pointer',
                  fontSize: 13,
                  borderRadius: 6,
                  transition: 'all 0.1s',
                  textAlign: 'left',
                }}
                onMouseEnter={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.background = 'rgba(88,166,255,0.1)';
                  (e.currentTarget as HTMLButtonElement).style.borderColor = 'rgba(88,166,255,0.2)';
                }}
                onMouseLeave={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.background = 'none';
                  (e.currentTarget as HTMLButtonElement).style.borderColor = 'transparent';
                }}
              >
                <span style={{ display: 'flex', alignItems: 'center', color: '#58a6ff', width: 20, justifyContent: 'center', flexShrink: 0 }}>{app.icon}</span>
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
