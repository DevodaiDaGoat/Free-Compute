'use client';

import { ReactNode, useState, useCallback } from 'react';
import Taskbar from '../taskbar/Taskbar';
import WindowManager from '../window-manager/WindowManager';
import type { AppWindow } from '../system/types';
import BrowserApp from '../apps/browser/Browser';
import TerminalApp from '../apps/terminal/Terminal';
import FilesApp from '../apps/files/Files';
import SettingsApp from '../apps/settings/Settings';
import CalculatorApp from '../apps/calculator/Calculator';
import AdminApp from '../apps/admin/Admin';
import ConnectionSettingsContent from '../apps/settings/ConnectionSettings';
import { defaultConnectionConfig } from '../system/types';

function ConnectionSettingsApp() {
  const [config, setConfig] = useState(defaultConnectionConfig());
  return <ConnectionSettingsContent config={config} onChange={setConfig} />;
}

const apps = [
  { id: 'browser', name: 'Browser', icon: '🌐' },
  { id: 'terminal', name: 'Terminal', icon: '⬛' },
  { id: 'files', name: 'Files', icon: '📁' },
  { id: 'settings', name: 'Settings', icon: '⚙️' },
  { id: 'connection', name: 'Connection', icon: '🔗' },
  { id: 'admin', name: 'Admin Panel', icon: '🔒' },
  { id: 'store', name: 'App Store', icon: '🛒' },
  { id: 'calculator', name: 'Calculator', icon: '🔢' },
  { id: 'task-manager', name: 'Task Manager', icon: '📊' },
];

const appComponents: Record<string, () => ReactNode> = {
  browser: () => <BrowserApp />,
  terminal: () => <TerminalApp />,
  files: () => <FilesApp />,
  settings: () => <SettingsApp />,
  connection: () => <ConnectionSettingsApp />,
  calculator: () => <CalculatorApp />,
  admin: () => <AdminApp />,
};

export default function Desktop() {
  const [windows, setWindows] = useState<AppWindow[]>([]);
  const [nextZ, setNextZ] = useState(10);

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
        zIndex: nextZ,
        minimized: false,
        maximized: false,
        focused: true,
      },
    ]);
    setNextZ((z) => z + 1);
  }, [windows, nextZ]);

  const bringToFront = useCallback((id: string) => {
    setWindows((prev) => {
      const w = prev.find((x) => x.id === id);
      if (!w) return prev;
      return prev.map((x) => ({ ...x, focused: x.id === id, zIndex: x.id === id ? nextZ : x.zIndex }));
    });
    setNextZ((z) => z + 1);
  }, [nextZ]);

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
      background: '#0f1923',
      position: 'relative',
      overflow: 'hidden',
      display: 'flex',
      flexDirection: 'column',
    }}>
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
