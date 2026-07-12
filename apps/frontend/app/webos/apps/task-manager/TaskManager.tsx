'use client';

import { X } from 'lucide-react';
import type { AppWindow } from '../../system/types';

interface TaskManagerProps {
  windows: AppWindow[];
  onClose: (id: string) => void;
}

export default function TaskManager({ windows, onClose }: TaskManagerProps) {
  if (windows.length === 0) {
    return (
      <div style={{
        padding: 32,
        color: '#484f58',
        textAlign: 'center',
        fontSize: 13,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 8,
      }}>
        <span style={{ fontSize: 28 }}>🗂</span>
        No apps are currently open.
      </div>
    );
  }

  return (
    <div style={{ padding: 16, height: '100%', overflow: 'auto' }}>
      <div style={{ fontSize: 11, color: '#8b949e', marginBottom: 10, textTransform: 'uppercase', letterSpacing: 1 }}>
        {windows.length} open window{windows.length !== 1 ? 's' : ''}
      </div>
      {windows.map((win) => (
        <div
          key={win.id}
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '8px 12px',
            marginBottom: 4,
            background: win.focused ? 'rgba(88,166,255,0.08)' : 'rgba(255,255,255,0.03)',
            border: `1px solid ${win.focused ? 'rgba(88,166,255,0.25)' : 'rgba(255,255,255,0.07)'}`,
            borderRadius: 6,
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <div style={{
              width: 7,
              height: 7,
              borderRadius: '50%',
              background: win.minimized ? '#6e7681' : win.focused ? '#58a6ff' : '#3fb950',
            }} />
            <span style={{ fontSize: 13, color: '#c9d1d9' }}>{win.title}</span>
            {win.minimized && (
              <span style={{ fontSize: 10, color: '#6e7681', marginLeft: 4 }}>minimized</span>
            )}
          </div>
          <button
            onClick={() => onClose(win.id)}
            aria-label={`Close ${win.title}`}
            style={{
              background: 'transparent',
              border: 'none',
              cursor: 'pointer',
              color: '#6e7681',
              display: 'flex',
              alignItems: 'center',
              padding: 4,
              borderRadius: 4,
              transition: 'color 0.15s',
            }}
            onMouseEnter={(e) => (e.currentTarget.style.color = '#f85149')}
            onMouseLeave={(e) => (e.currentTarget.style.color = '#6e7681')}
          >
            <X size={14} />
          </button>
        </div>
      ))}
    </div>
  );
}
