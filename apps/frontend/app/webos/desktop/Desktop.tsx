'use client';

import { ReactNode, useState, useCallback, useRef, useEffect, useMemo } from 'react';
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
import TaskManager from '../apps/task-manager/TaskManager';
import HostMonitor from '../apps/host-monitor/HostMonitor';
import CreditsApp from '../apps/credits/Credits';
import AIMonitorApp from '../apps/ai-monitor/AIMonitor';
import { defaultConnectionConfig } from '../system/types';
import { getGatewayUrl } from '../boot/BootSequence';
import {
  Monitor,
  Terminal as TerminalIcon,
  Folder,
  Settings as SettingsIcon,
  Link2,
  Shield,
  Store,
  Calculator,
  BarChart2,
  Package2,
  Server,
  Wifi,
  WifiOff,
  CreditCard,
  Brain,
} from 'lucide-react';

function ConnectionSettingsApp() {
  const [config, setConfig] = useState(defaultConnectionConfig());
  return <ConnectionSettingsContent config={config} onChange={setConfig} />;
}

interface DockEntry {
  id: string;
  name: string;
  icon: ReactNode;
  color: string;
}

const DOCK_APPS: DockEntry[] = [
  { id: 'browser',        name: 'Browser',        icon: <Monitor size={26} />,      color: '#1abc9c' },
  { id: 'terminal',       name: 'Terminal',        icon: <TerminalIcon size={26} />, color: '#3498db' },
  { id: 'files',          name: 'Files',           icon: <Folder size={26} />,       color: '#e67e22' },
  { id: 'remote-desktop', name: 'Remote Desktop',  icon: <Monitor size={26} />,      color: '#9b59b6' },
  { id: 'connection',     name: 'Connection',      icon: <Link2 size={26} />,        color: '#e74c3c' },
  { id: 'settings',       name: 'Settings',        icon: <SettingsIcon size={26} />, color: '#95a5a6' },
  { id: 'task-manager',   name: 'Task Manager',    icon: <BarChart2 size={26} />,    color: '#d29922' },
  { id: 'host-monitor',   name: 'Host Monitor',    icon: <Server size={26} />,       color: '#58a6ff' },
  { id: 'credits',        name: 'Credits',         icon: <CreditCard size={26} />,   color: '#d2a8ff' },
  { id: 'ai-monitor',    name: 'AI Monitor',      icon: <Brain size={26} />,         color: '#ffa657' },
  { id: 'store',          name: 'App Store',       icon: <Store size={26} />,        color: '#2ecc71' },
  { id: 'calculator',     name: 'Calculator',      icon: <Calculator size={26} />,   color: '#e67e22' },
  { id: 'admin',          name: 'Admin',           icon: <Shield size={26} />,       color: '#f85149' },
  { id: 'app-player',     name: 'App Player',      icon: <Package2 size={26} />,     color: '#8957e8' },
];

const ALL_APPS = [
  { id: 'browser',        name: 'Browser',        icon: <Monitor size={18} /> },
  { id: 'terminal',       name: 'Terminal',        icon: <TerminalIcon size={18} /> },
  { id: 'files',          name: 'Files',           icon: <Folder size={18} /> },
  { id: 'settings',       name: 'Settings',        icon: <SettingsIcon size={18} /> },
  { id: 'connection',     name: 'Connection',      icon: <Link2 size={18} /> },
  { id: 'admin',          name: 'Admin Panel',     icon: <Shield size={18} /> },
  { id: 'store',          name: 'App Store',       icon: <Store size={18} /> },
  { id: 'calculator',     name: 'Calculator',      icon: <Calculator size={18} /> },
  { id: 'task-manager',   name: 'Task Manager',    icon: <BarChart2 size={18} /> },
  { id: 'host-monitor',   name: 'Host Monitor',    icon: <Server size={18} /> },
  { id: 'credits',        name: 'Credits',         icon: <CreditCard size={18} /> },
  { id: 'ai-monitor',    name: 'AI Monitor',      icon: <Brain size={18} /> },
  { id: 'app-player',     name: 'App Player',      icon: <Package2 size={18} /> },
  { id: 'remote-desktop', name: 'Remote Desktop',  icon: <Monitor size={18} /> },
];

function useClock() {
  const [time, setTime] = useState(() => new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }));
  const [date, setDate] = useState(() => new Date().toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' }));
  useEffect(() => {
    const id = setInterval(() => {
      const now = new Date();
      setTime(now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }));
      setDate(now.toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' }));
    }, 10000);
    return () => clearInterval(id);
  }, []);
  return { time, date };
}

function useGatewayPing() {
  const [online, setOnline] = useState<boolean | null>(null);
  useEffect(() => {
    let active = true;
    const controller = new AbortController();
    const check = () =>
      fetch(`${getGatewayUrl()}/healthz`, { signal: controller.signal })
        .then((r) => { if (active) setOnline(r.ok); })
        .catch(() => { if (active) setOnline(false); });
    check();
    const id = setInterval(check, 15000);
    return () => { active = false; controller.abort(); clearInterval(id); };
  }, []);
  return online;
}

function MenuBar({
  focusedTitle,
  online,
  time,
  date,
  onOpenLauncher,
}: {
  focusedTitle: string;
  online: boolean | null;
  time: string;
  date: string;
  onOpenLauncher: () => void;
}) {
  return (
    <div style={{
      height: 28,
      background: 'rgba(10,14,22,0.88)',
      borderBottom: '1px solid rgba(255,255,255,0.06)',
      backdropFilter: 'blur(20px) saturate(180%)',
      display: 'flex',
      alignItems: 'center',
      padding: '0 14px',
      zIndex: 9999,
      flexShrink: 0,
      userSelect: 'none',
    }}>
      <button
        onClick={onOpenLauncher}
        style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '0 8px 0 0', display: 'flex', alignItems: 'center', gap: 5, color: '#e6edf3', fontSize: 13, fontWeight: 700 }}
      >
        <Server size={13} color="#58a6ff" />
        FreeCompute
      </button>

      <div style={{ width: 1, height: 14, background: 'rgba(255,255,255,0.1)', margin: '0 10px' }} />

      <span style={{ fontSize: 12, fontWeight: 600, color: '#c9d1d9', flex: 1 }}>
        {focusedTitle || 'Desktop'}
      </span>

      <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 12, color: online === true ? '#3fb950' : online === false ? '#f85149' : '#6e7681' }}>
          {online ? <Wifi size={12} /> : <WifiOff size={12} />}
          <span style={{ fontSize: 11 }}>{online === true ? 'Online' : online === false ? 'Offline' : '...'}</span>
        </span>
        <span style={{ fontSize: 12, color: '#8b949e' }}>{date}</span>
        <span style={{ fontSize: 12, fontWeight: 600, color: '#e6edf3' }}>{time}</span>
      </div>
    </div>
  );
}

function Dock({
  apps,
  openWindowIds,
  focusedWindowApp,
  onOpen,
  onSwitchToApp,
}: {
  apps: DockEntry[];
  openWindowIds: Record<string, string[]>;
  focusedWindowApp: string | null;
  onOpen: (id: string) => void;
  onSwitchToApp: (id: string) => void;
}) {
  const [hovered, setHovered] = useState<string | null>(null);

  return (
    <div style={{
      position: 'absolute',
      bottom: 10,
      left: '50%',
      transform: 'translateX(-50%)',
      zIndex: 9998,
      display: 'flex',
      alignItems: 'flex-end',
      gap: 6,
      padding: '8px 14px',
      background: 'rgba(20,24,32,0.75)',
      border: '1px solid rgba(255,255,255,0.1)',
      borderRadius: 18,
      backdropFilter: 'blur(24px) saturate(180%)',
      boxShadow: '0 8px 40px rgba(0,0,0,0.7), 0 1px 0 rgba(255,255,255,0.06) inset',
    }}>
      {apps.map((app) => {
        const isOpen = (openWindowIds[app.id]?.length ?? 0) > 0;
        const isFocused = focusedWindowApp === app.id;
        const isHov = hovered === app.id;
        const scale = isHov ? 1.35 : 1;
        return (
          <div
            key={app.id}
            style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 4, position: 'relative' }}
            onMouseEnter={() => setHovered(app.id)}
            onMouseLeave={() => setHovered(null)}
          >
            {isHov && (
              <div style={{
                position: 'absolute',
                bottom: 'calc(100% + 6px)',
                left: '50%',
                transform: 'translateX(-50%)',
                background: 'rgba(0,0,0,0.85)',
                color: '#e6edf3',
                fontSize: 11,
                fontWeight: 600,
                padding: '4px 10px',
                borderRadius: 6,
                whiteSpace: 'nowrap',
                pointerEvents: 'none',
                border: '1px solid rgba(255,255,255,0.1)',
              }}>
                {app.name}
              </div>
            )}
            <button
              onClick={() => isOpen ? onSwitchToApp(app.id) : onOpen(app.id)}
              aria-label={app.name}
              title={app.name}
              style={{
                width: 48,
                height: 48,
                borderRadius: 12,
                background: isFocused
                  ? `${app.color}30`
                  : isHov
                  ? 'rgba(255,255,255,0.12)'
                  : 'rgba(255,255,255,0.06)',
                border: `1px solid ${isFocused ? `${app.color}60` : 'rgba(255,255,255,0.1)'}`,
                color: app.color,
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                transition: 'transform 0.12s ease, background 0.12s, border-color 0.12s',
                transform: `scale(${scale}) translateY(${isHov ? -6 : 0}px)`,
                boxShadow: isFocused ? `0 0 14px ${app.color}40` : 'none',
                padding: 0,
              }}
            >
              {app.icon}
            </button>
            {isOpen && (
              <div style={{ width: 4, height: 4, borderRadius: '50%', background: isFocused ? app.color : 'rgba(255,255,255,0.35)' }} />
            )}
          </div>
        );
      })}
    </div>
  );
}

function AppLauncher({ onOpen, onClose }: { onOpen: (id: string) => void; onClose: () => void }) {
  const [search, setSearch] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);
  useEffect(() => { inputRef.current?.focus(); }, []);

  const filtered = ALL_APPS.filter((a) => a.name.toLowerCase().includes(search.toLowerCase()));

  return (
    <>
      <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.6)', backdropFilter: 'blur(8px)', zIndex: 10000 }} onClick={onClose} />
      <div style={{
        position: 'fixed',
        top: '50%',
        left: '50%',
        transform: 'translate(-50%, -50%)',
        width: 480,
        maxWidth: '90vw',
        background: 'rgba(18,24,36,0.96)',
        border: '1px solid rgba(88,166,255,0.2)',
        borderRadius: 16,
        boxShadow: '0 24px 80px rgba(0,0,0,0.8)',
        zIndex: 10001,
        overflow: 'hidden',
      }}>
        <div style={{ padding: '16px 16px 0' }}>
          <input
            ref={inputRef}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search apps..."
            style={{
              width: '100%',
              padding: '10px 14px',
              background: 'rgba(255,255,255,0.07)',
              border: '1px solid rgba(88,166,255,0.25)',
              borderRadius: 10,
              color: '#e6edf3',
              fontSize: 14,
              outline: 'none',
              boxSizing: 'border-box',
            }}
          />
        </div>
        <div style={{ padding: 12, display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 8 }}>
          {filtered.map((app) => (
            <button
              key={app.id}
              onClick={() => { onOpen(app.id); onClose(); }}
              style={{
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                gap: 6,
                padding: '12px 8px',
                background: 'rgba(255,255,255,0.04)',
                border: '1px solid rgba(255,255,255,0.07)',
                borderRadius: 10,
                cursor: 'pointer',
                color: '#c9d1d9',
                transition: 'background 0.1s',
              }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLElement).style.background = 'rgba(88,166,255,0.12)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLElement).style.background = 'rgba(255,255,255,0.04)'; }}
            >
              <span style={{ color: '#58a6ff' }}>{app.icon}</span>
              <span style={{ fontSize: 11, fontWeight: 600, textAlign: 'center', lineHeight: 1.2 }}>{app.name}</span>
            </button>
          ))}
        </div>
      </div>
    </>
  );
}

function AltTabSwitcher({
  windows,
  selectedId,
  onSelect,
}: {
  windows: AppWindow[];
  selectedId: string;
  onSelect: (id: string) => void;
}) {
  const visible = windows.filter((w) => !w.minimized);
  return (
    <div style={{
      position: 'fixed',
      inset: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      zIndex: 10002,
      background: 'rgba(0,0,0,0.5)',
      backdropFilter: 'blur(6px)',
    }}>
      <div style={{
        display: 'flex',
        gap: 12,
        padding: '20px 24px',
        background: 'rgba(18,24,36,0.9)',
        border: '1px solid rgba(255,255,255,0.12)',
        borderRadius: 16,
        boxShadow: '0 16px 64px rgba(0,0,0,0.8)',
        maxWidth: '80vw',
        flexWrap: 'wrap',
        justifyContent: 'center',
      }}>
        {visible.length === 0 && (
          <span style={{ fontSize: 13, color: '#6e7681', padding: 8 }}>No open windows</span>
        )}
        {visible.map((w) => (
          <button
            key={w.id}
            onClick={() => onSelect(w.id)}
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              gap: 8,
              padding: '14px 18px',
              borderRadius: 12,
              background: w.id === selectedId ? 'rgba(88,166,255,0.2)' : 'rgba(255,255,255,0.05)',
              border: `1px solid ${w.id === selectedId ? 'rgba(88,166,255,0.5)' : 'rgba(255,255,255,0.08)'}`,
              cursor: 'pointer',
              minWidth: 90,
              color: '#c9d1d9',
              transition: 'background 0.1s',
            }}
          >
            <span style={{ fontSize: 28 }}>
              {ALL_APPS.find((a) => a.id === w.app)?.icon ?? <Monitor size={28} />}
            </span>
            <span style={{ fontSize: 11, fontWeight: 600, color: w.id === selectedId ? '#58a6ff' : '#8b949e', maxWidth: 90, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {w.title}
            </span>
          </button>
        ))}
      </div>
    </div>
  );
}

export default function Desktop() {
  const [windows, setWindows] = useState<AppWindow[]>([]);
  const [launcherOpen, setLauncherOpen] = useState(false);
  const [altTabOpen, setAltTabOpen] = useState(false);
  const [altTabIndex, setAltTabIndex] = useState(0);
  const nextZRef = useRef(10);
  const { time, date } = useClock();
  const online = useGatewayPing();

  const focusedWindow = windows.find((w) => w.focused && !w.minimized) ?? null;
  const focusedTitle = focusedWindow?.title ?? '';

  const openWindowIds = useMemo(() => {
    const map: Record<string, string[]> = {};
    for (const w of windows) {
      if (!w.minimized) {
        if (!map[w.app]) map[w.app] = [];
        map[w.app].push(w.id);
      }
    }
    return map;
  }, [windows]);

  const focusedWindowApp = focusedWindow?.app ?? null;

  const openApp = useCallback((appId: string) => {
    const app = ALL_APPS.find((a) => a.id === appId);
    if (!app) return;
    setWindows((prev) => {
      const existing = prev.find((w) => w.app === appId && !w.minimized);
      if (existing) {
        const newZ = (nextZRef.current += 1);
        return prev.map((w) => ({ ...w, focused: w.id === existing.id, zIndex: w.id === existing.id ? newZ : w.zIndex }));
      }
      const z = (nextZRef.current += 1);
      return [
        ...prev.map((w) => ({ ...w, focused: false })),
        {
          id: `win-${Date.now()}`,
          title: app.name,
          app: appId,
          x: 60 + (prev.length % 8) * 28,
          y: 40 + (prev.length % 8) * 28,
          width: 900,
          height: 560,
          zIndex: z,
          minimized: false,
          maximized: false,
          focused: true,
        },
      ];
    });
  }, []);

  const switchToApp = useCallback((appId: string) => {
    setWindows((prev) => {
      const target = prev.find((w) => w.app === appId && !w.minimized);
      if (!target) return prev;
      const newZ = (nextZRef.current += 1);
      return prev.map((w) => ({ ...w, focused: w.id === target.id, zIndex: w.id === target.id ? newZ : w.zIndex }));
    });
  }, []);

  const bringToFront = useCallback((id: string) => {
    setWindows((prev) => {
      const newZ = (nextZRef.current += 1);
      return prev.map((w) => ({
        ...w,
        focused: w.id === id,
        minimized: w.id === id ? false : w.minimized,
        zIndex: w.id === id ? newZ : w.zIndex,
      }));
    });
  }, []);

  const closeWindow = useCallback((id: string) => {
    setWindows((prev) => prev.filter((w) => w.id !== id));
  }, []);

  const minimizeWindow = useCallback((id: string) => {
    setWindows((prev) => prev.map((w) => w.id === id ? { ...w, minimized: true, focused: false } : w));
  }, []);

  const toggleMaximize = useCallback((id: string) => {
    setWindows((prev) => prev.map((w) => w.id === id ? { ...w, maximized: !w.maximized } : w));
  }, []);

  const updatePosition = useCallback((id: string, x: number, y: number) => {
    setWindows((prev) => prev.map((w) => w.id === id ? { ...w, x, y } : w));
  }, []);

  const updateSize = useCallback((id: string, width: number, height: number) => {
    setWindows((prev) => prev.map((w) => w.id === id ? { ...w, width, height } : w));
  }, []);

  const visibleForAltTab = useMemo(() => windows.filter((w) => !w.minimized), [windows]);

  const confirmAltTab = useCallback(() => {
    if (!visibleForAltTab.length) return;
    const target = visibleForAltTab[altTabIndex % visibleForAltTab.length];
    if (target) bringToFront(target.id);
    setAltTabOpen(false);
  }, [visibleForAltTab, altTabIndex, bringToFront]);

  useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if ((e.altKey || e.metaKey) && e.key === 'Tab') {
        e.preventDefault();
        if (!altTabOpen) {
          setAltTabOpen(true);
          setAltTabIndex(1 % Math.max(1, visibleForAltTab.length));
        } else {
          setAltTabIndex((i) => (i + 1) % Math.max(1, visibleForAltTab.length));
        }
      }
      if (e.key === 'Escape' && altTabOpen) {
        setAltTabOpen(false);
      }
      if ((e.metaKey || e.ctrlKey) && e.key === ' ') {
        e.preventDefault();
        setLauncherOpen((v) => !v);
      }
    };
    const up = (e: KeyboardEvent) => {
      if (altTabOpen && (e.key === 'Alt' || e.key === 'Meta')) {
        confirmAltTab();
      }
    };
    window.addEventListener('keydown', down);
    window.addEventListener('keyup', up);
    return () => {
      window.removeEventListener('keydown', down);
      window.removeEventListener('keyup', up);
    };
  }, [altTabOpen, visibleForAltTab.length, confirmAltTab]);

  // Keep a ref to the latest windows/closeWindow so TaskManager can read the
  // current data without appComponents' identity changing (which would remount
  // every app and wipe internal state — like the Credits tab).
  const windowsRef = useRef(windows);
  const closeWindowRef = useRef(closeWindow);
  windowsRef.current = windows;
  closeWindowRef.current = closeWindow;

  const TaskManagerLive = useCallback(() => (
    <TaskManager windows={windowsRef.current} onClose={(id) => closeWindowRef.current(id)} />
  ), []);

  const appComponents: Record<string, () => ReactNode> = useMemo(() => ({
    browser:          () => <BrowserApp />,
    terminal:         () => <TerminalApp />,
    files:            () => <FilesApp />,
    settings:         () => <SettingsApp />,
    connection:       () => <ConnectionSettingsApp />,
    calculator:       () => <CalculatorApp />,
    admin:            () => <AdminApp />,
    store:            () => <AppPlayerApp />,
    'app-player':     () => <AppPlayerApp />,
    'remote-desktop': () => <RemoteDesktopApp />,
    'task-manager':   () => <TaskManagerLive />,
    'host-monitor':   () => <HostMonitor />,
    'credits':        () => <CreditsApp />,
    'ai-monitor':     () => <AIMonitorApp />,
  }), [TaskManagerLive]);

  return (
    <div style={{
      height: '100vh',
      width: '100vw',
      display: 'flex',
      flexDirection: 'column',
      background: 'linear-gradient(160deg, #080c18 0%, #0a0f1e 40%, #060d1a 100%)',
      overflow: 'hidden',
      position: 'relative',
    }}>
      {/* Subtle grid */}
      <div aria-hidden style={{
        position: 'absolute',
        inset: 0,
        backgroundImage:
          'linear-gradient(rgba(88,166,255,0.025) 1px, transparent 1px), linear-gradient(90deg, rgba(88,166,255,0.025) 1px, transparent 1px)',
        backgroundSize: '48px 48px',
        pointerEvents: 'none',
      }} />

      {/* Ambient light */}
      <div aria-hidden style={{
        position: 'absolute',
        top: '20%',
        left: '30%',
        width: '40%',
        height: '40%',
        background: 'radial-gradient(circle, rgba(88,166,255,0.04) 0%, transparent 70%)',
        pointerEvents: 'none',
      }} />

      {/* Menu bar */}
      <MenuBar
        focusedTitle={focusedTitle}
        online={online}
        time={time}
        date={date}
        onOpenLauncher={() => setLauncherOpen(true)}
      />

      {/* Desktop work area */}
      <div style={{ flex: 1, position: 'relative', overflow: 'hidden' }}>
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
        <Dock
          apps={DOCK_APPS}
          openWindowIds={openWindowIds}
          focusedWindowApp={focusedWindowApp}
          onOpen={openApp}
          onSwitchToApp={switchToApp}
        />
      </div>

      {launcherOpen && (
        <AppLauncher
          onOpen={openApp}
          onClose={() => setLauncherOpen(false)}
        />
      )}

      {altTabOpen && (
        <AltTabSwitcher
          windows={visibleForAltTab}
          selectedId={visibleForAltTab[altTabIndex % Math.max(1, visibleForAltTab.length)]?.id ?? ''}
          onSelect={(id) => { bringToFront(id); setAltTabOpen(false); }}
        />
      )}

      <style>{`
        @keyframes spin { to { transform: rotate(360deg); } }
        .spin { animation: spin 0.9s linear infinite; }
      `}</style>
    </div>
  );
}
