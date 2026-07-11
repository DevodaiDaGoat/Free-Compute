'use client';

import { ReactNode, useState, useCallback, useRef } from 'react';
import Taskbar from '../taskbar/Taskbar';
import WindowManager from '../window-manager/WindowManager';
import type { AppWindow } from '../system/types';
import RemoteDesktopApp from '../apps/remote-desktop/RemoteDesktop';
import BrowserApp from '../apps/browser/Browser';
import TerminalApp from '../apps/terminal/Terminal';
import FilesApp from '../apps/files/Files';
import SettingsApp from '../apps/settings/Settings';
import CalculatorApp from '../apps/calculator/Calculator';
import AdminApp from '../apps/admin/Admin';
import ConnectionSettingsContent from '../apps/settings/ConnectionSettings';
import AppPlayerApp from '../apps/store/AppPlayer';
import { defaultConnectionConfig } from '../system/types';
import { Monitor, Terminal as TerminalIcon, Folder, Settings as SettingsIcon, Link2, Shield, Store, Calculator, BarChart2, Package2 } from 'lucide-react';

function ConnectionSettingsApp() {
  const [config, setConfig] = useState(defaultConnectionConfig());
  return <ConnectionSettingsContent config={config} onChange={setConfig} />;
}

const apps = [
  { id: 'browser', name: 'Browser', icon: <Monitor size={20} /> },
  { id: 'terminal', name: 'Terminal', icon: <TerminalIcon size={20} /> },
  { id: 'files', name: 'Files', icon: <Folder size={20} /> },
  { id: 'settings', name: 'Settings', icon: <SettingsIcon size={20} /> },
  { id: 'connection', name: 'Connection', icon: <Link2 size={20} /> },
  { id: 'admin', name: 'Admin Panel', icon: <Shield size={20} /> },
  { id: 'store', name: 'App Store', icon: <Store size={20} /> },
  { id: 'calculator', name: 'Calculator', icon: <Calculator size={20} /> },
  { id: 'task-manager', name: 'Task Manager', icon: <BarChart2 size={20} /> },
  { id: 'app-player', name: 'App Player', icon: <Package2 size={20} /> },
  { id: 'remote-desktop', name: 'Remote Desktop', icon: <Monitor size={20} /> },
];

const appComponents: Record<string, () => ReactNode> = {
  browser: () => <BrowserApp />,
  terminal: () => <TerminalApp />,
  files: () => <FilesApp />,
  settings: () => <SettingsApp />,
  connection: () => <ConnectionSettingsApp />,
  calculator: () => <CalculatorApp />,
  admin: () => <AdminApp />,
  store: () => null,
  'app-player': () => <AppPlayerApp />,
  'remote-desktop': () => <RemoteDesktopApp />,
};

function DesktopWidgets({ onOpenApp }: { onOpenApp: (id: string) => void }) {
  return (
    <div style={{ position: 'absolute', top: 16, right: 16, display: 'flex', gap: 12, zIndex: 1 }}>
      <div style={{ background: 'rgba(17,17,40,0.85)', border: '1px solid #2a2a4a', borderRadius: 8, padding: 12, minWidth: 140 }}>
        <div style={{ fontSize: 10, color: '#888', marginBottom: 4 }}>System Status</div>
        <div style={{ fontSize: 12, color: '#18e2ff' }}>Gateway: <span style={{ color: '#238636' }}>● Online</span></div>
        <div style={{ fontSize: 12, color: '#ccc' }}>Storage: 2.4 / 10 GB used</div>
      </div>
      <div style={{ background: 'rgba(17,17,40,0.85)', border: '1px solid #2a2a4a', borderRadius: 8, padding: 12, minWidth: 140 }}>
        <div style={{ fontSize: 10, color: '#888', marginBottom: 4 }}>Quick Launch</div>
        <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
          <button onClick={() => onOpenApp('browser')} style={{ background: 'rgba(26,188,156,0.15)', border: '1px solid rgba(26,188,156,0.3)', borderRadius: 4, padding: 6, cursor: 'pointer', display: 'flex', alignItems: 'center', color: '#1abc9c' }}><Monitor size={16} /></button>
          <button onClick={() => onOpenApp('terminal')} style={{ background: 'rgba(52,152,219,0.15)', border: '1px solid rgba(52,152,219,0.3)', borderRadius: 4, padding: 6, cursor: 'pointer', display: 'flex', alignItems: 'center', color: '#3498db' }}><TerminalIcon size={16} /></button>
          <button onClick={() => onOpenApp('files')} style={{ background: 'rgba(230,126,34,0.15)', border: '1px solid rgba(230,126,34,0.3)', borderRadius: 4, padding: 6, cursor: 'pointer', display: 'flex', alignItems: 'center', color: '#e67e22' }}><Folder size={16} /></button>
          <button onClick={() => onOpenApp('calculator')} style={{ background: 'rgba(155,89,182,0.15)', border: '1px solid rgba(155,89,182,0.3)', borderRadius: 4, padding: 6, cursor: 'pointer', display: 'flex', alignItems: 'center', color: '#9b59b6' }}><Calculator size={16} /></button>
        </div>
      </div>
    </div>
  );
}

export default function Desktop() {
  const [windows, setWindows] = useState<AppWindow[]>([]);
  const nextZRef = useRef(10);

  const openApp = useCallback((appId: string) => {
    const app = apps.find((a) => a.id === appId);
    if (!app) return;
    const existing = windows.find((w) => w.app === appId && !w.minimized);
    if (existing) {
      bringToFront(existing.id);
      return;
    }
    setWindows((prev) => [
      ...prev.map((w) => ({ ...w, focused: false })),
      {
        id: `win-${Date.now()}`,
        title: app.name,
        app: appId,
        x: 100 + (prev.length * 30),
        y: 60 + (prev.length * 30),
        width: 800,
        height: 500,
        zIndex: nextZRef.current,
        minimized: false,
        maximized: false,
        focused: true,
      },
    ]);
    nextZRef.current += 1;
  }, [windows]);

  const bringToFront = useCallback((id: string) => {
    setWindows((prev) => {
      const w = prev.find((x) => x.id === id);
      if (!w) return prev;
      const newZ = nextZRef.current + 1;
      nextZRef.current = newZ;
      return prev.map((x) => ({ ...x, focused: x.id === id, zIndex: x.id === id ? newZ : x.zIndex }));
    });
  }, []);

  const closeWindow = useCallback((id: string) => {
    setWindows((prev) => prev.filter((w) => w.id !== id));
  }, []);

  const minimizeWindow = useCallback((id: string) => {
    setWindows((prev) => prev.map((w) => w.id === id ? { ...w, minimized: true, focused: false } : w));
  }, []);

  const toggleMaximize = useCallback((id: string) => {
    setWindows((prev) =>
      prev.map((w) => w.id === id ? { ...w, maximized: !w.maximized } : w)
    );
  }, []);

  const updatePosition = useCallback((id: string, x: number, y: number) => {
    setWindows((prev) => prev.map((w) => w.id === id ? { ...w, x, y } : w));
  }, []);

  const updateSize = useCallback((id: string, width: number, height: number) => {
    setWindows((prev) => prev.map((w) => w.id === id ? { ...w, width, height } : w));
  }, []);

  return (
    <div style={{
      height: '100vh',
      width: '100vw',
      background: 'linear-gradient(135deg, #0f172a 0%, #1e293b 50%, #0f1923 100%)',
      position: 'relative',
      overflow: 'hidden',
      display: 'flex',
      flexDirection: 'column',
    }}>
      <DesktopWidgets onOpenApp={openApp} />
      <div style={{ flex: 1, position: 'relative' }}>
        <WindowManager
          windows={windows}
          appComponents={appComponents}
          onFocus={bringToFront}
          onClose={closeWindow}
          onMinimize={minimizeWindow}
          onMaximize={toggleMaximize}
          onMove={updatePosition}
          onResize={updateSize}
        />
      </div>
      <Taskbar windows={windows} apps={apps} onOpenApp={openApp} onFocus={bringToFront} />
    </div>
  );
}
