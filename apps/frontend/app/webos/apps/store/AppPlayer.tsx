'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { Package2, Play, Trash2, Upload, X, Terminal as TerminalIcon, CheckCircle2 } from 'lucide-react';

type PkgStatus = 'idle' | 'installing' | 'installed' | 'running' | 'error';

interface Pkg {
  id: string;
  name: string;
  type: 'deb' | 'exe' | 'appimage' | 'unknown';
  size: string;
  description: string;
  status: PkgStatus;
}

const CATALOG: Pkg[] = [
  { id: 'nano',  name: 'nano 7.2',         type: 'deb', size: '245 KB', description: 'GNU nano text editor',                   status: 'idle' },
  { id: 'htop',  name: 'htop 3.3.0',       type: 'deb', size: '128 KB', description: 'Interactive process viewer',              status: 'idle' },
  { id: 'vim',   name: 'vim 9.1',           type: 'deb', size: '3.2 MB', description: 'Highly configurable text editor',         status: 'idle' },
  { id: 'curl',  name: 'curl 8.5.0',        type: 'deb', size: '1.1 MB', description: 'Command-line HTTP client',                status: 'idle' },
  { id: 'git',   name: 'git 2.43.0',        type: 'deb', size: '52 MB',  description: 'Distributed version control',             status: 'idle' },
  { id: 'node',  name: 'Node.js 20.11 LTS', type: 'deb', size: '78 MB',  description: 'JavaScript runtime (npm included)',       status: 'idle' },
  { id: 'python', name: 'Python 3.12',        type: 'deb', size: '22 MB',  description: 'Python interpreter + pip',                status: 'idle' },
  { id: 'code',  name: 'VS Code 1.88',       type: 'deb', size: '96 MB',  description: 'Microsoft Visual Studio Code',            status: 'idle' },
];

function pkgColor(type: Pkg['type']) {
  if (type === 'deb')      return '#3fb950';
  if (type === 'exe')      return '#58a6ff';
  if (type === 'appimage') return '#d2a8ff';
  return '#6e7681';
}

function statusLabel(s: PkgStatus) {
  switch (s) {
    case 'installing': return 'Installing...';
    case 'installed':  return 'Installed';
    case 'running':    return 'Running';
    case 'error':      return 'Error';
    default:           return '';
  }
}

export default function AppPlayerApp() {
  const [pkgs, setPkgs] = useState<Pkg[]>(CATALOG.map((p) => ({ ...p })));
  const [lines, setLines] = useState<string[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const consoleRef = useRef<HTMLDivElement>(null);
  const timersRef = useRef<Set<ReturnType<typeof setTimeout>>>(new Set());
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      for (const t of timersRef.current) clearTimeout(t);
      timersRef.current.clear();
    };
  }, []);

  const schedule = useCallback((fn: () => void, delay: number) => {
    const id = setTimeout(() => {
      timersRef.current.delete(id);
      if (mountedRef.current) fn();
    }, delay);
    timersRef.current.add(id);
    return id;
  }, []);

  const log = useCallback((line: string) => {
    if (!mountedRef.current) return;
    setLines((prev) => {
      const next = [...prev, line];
      requestAnimationFrame(() => {
        if (consoleRef.current) consoleRef.current.scrollTop = consoleRef.current.scrollHeight;
      });
      return next;
    });
  }, []);

  const updatePkg = useCallback((id: string, patch: Partial<Pkg>) => {
    if (!mountedRef.current) return;
    setPkgs((prev) => prev.map((p) => p.id === id ? { ...p, ...patch } : p));
  }, []);

  const install = useCallback((pkg: Pkg) => {
    updatePkg(pkg.id, { status: 'installing' });
    log(`$ apt-get install -y ${pkg.name.split(' ')[0].toLowerCase()}`);
    log(`Reading package lists... Done`);
    log(`Building dependency tree... Done`);
    log(`The following NEW packages will be installed:`);
    log(`  ${pkg.name.toLowerCase().replace(' ', '_')}`);
    log(`0 upgraded, 1 newly installed, 0 to remove.`);
    log(`Need to get ${pkg.size} of archives.`);

    const steps = [
      `Get:1 http://archive.ubuntu.com/ubuntu jammy/main amd64 ${pkg.name.toLowerCase()} [${pkg.size}]`,
      `Fetched ${pkg.size} in 0.3s (1,230 kB/s)`,
      `Selecting previously unselected package ${pkg.name.split(' ')[0].toLowerCase()}.`,
      `(Reading database ... 137,482 files and directories currently installed.)`,
      `Preparing to unpack .../archives/${pkg.name.split(' ')[0].toLowerCase()}_amd64.deb ...`,
      `Unpacking ${pkg.name.split(' ')[0].toLowerCase()} ...`,
      `Setting up ${pkg.name.split(' ')[0].toLowerCase()} ...`,
      `Processing triggers for man-db ...`,
      `done.`,
    ];
    let i = 0;
    const tick = () => {
      if (!mountedRef.current) return;
      if (i < steps.length) {
        log(steps[i++]);
        schedule(tick, 120 + Math.random() * 80);
      } else {
        updatePkg(pkg.id, { status: 'installed' });
        log(`✓ ${pkg.name} installed successfully`);
      }
    };
    schedule(tick, 400);
  }, [log, updatePkg, schedule]);

  const run = useCallback((pkg: Pkg) => {
    updatePkg(pkg.id, { status: 'running' });
    log(`$ ${pkg.name.split(' ')[0].toLowerCase()}`);
    schedule(() => {
      log(`Loading ${pkg.name}...`);
      log(`Runtime: OK`);
      log(`Process started (PID ${10000 + Math.floor(Math.random() * 50000)})`);
      if (pkg.type === 'deb') log(`Memory footprint: ${Math.floor(Math.random() * 80 + 8)} MB`);
    }, 300);
  }, [log, updatePkg, schedule]);

  const remove = useCallback((pkg: Pkg) => {
    log(`$ apt-get remove -y ${pkg.name.split(' ')[0].toLowerCase()}`);
    log(`Removing ${pkg.name} ...`);
    schedule(() => {
      updatePkg(pkg.id, { status: 'idle' });
      log(`${pkg.name} removed.`);
    }, 600);
  }, [log, updatePkg, schedule]);

  const handleFileUpload = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const ext = file.name.split('.').pop()?.toLowerCase() ?? '';
    const type: Pkg['type'] = ext === 'deb' ? 'deb' : ext === 'exe' ? 'exe' : ext === 'appimage' ? 'appimage' : 'unknown';
    const newPkg: Pkg = {
      id: `upload-${Date.now()}`,
      name: file.name,
      type,
      size: file.size > 1024 * 1024 ? `${(file.size / 1024 / 1024).toFixed(1)} MB` : `${Math.round(file.size / 1024)} KB`,
      description: `Uploaded package`,
      status: 'idle',
    };
    setPkgs((prev) => [newPkg, ...prev]);
    setSelected(newPkg.id);
    log(`Uploaded: ${file.name} (${newPkg.size})`);
    if (fileInputRef.current) fileInputRef.current.value = '';
  }, [log]);

  const sel = pkgs.find((p) => p.id === selected);

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', background: '#0a0f1e', color: '#c9d1d9', fontFamily: 'system-ui, sans-serif', overflow: 'hidden' }}>
      {/* Header */}
      <div style={{ padding: '12px 16px', background: 'rgba(22,27,34,0.9)', borderBottom: '1px solid rgba(48,54,61,0.5)', display: 'flex', alignItems: 'center', gap: 10, flexShrink: 0 }}>
        <Package2 size={17} color="#8957e8" />
        <span style={{ fontSize: 14, fontWeight: 700, color: '#e6edf3' }}>App Player</span>
        <span style={{ marginLeft: 'auto' }}>
          <button
            onClick={() => fileInputRef.current?.click()}
            style={{ padding: '6px 12px', borderRadius: 6, background: 'rgba(137,87,232,0.15)', border: '1px solid rgba(137,87,232,0.3)', color: '#d2a8ff', cursor: 'pointer', fontSize: 12, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}
          >
            <Upload size={12} />
            Upload Package
          </button>
        </span>
        <input ref={fileInputRef} type="file" accept=".deb,.exe,.appimage" onChange={handleFileUpload} style={{ display: 'none' }} />
      </div>

      <div style={{ flex: 1, display: 'grid', gridTemplateColumns: '240px 1fr', overflow: 'hidden' }}>
        {/* Package list */}
        <div style={{ borderRight: '1px solid rgba(48,54,61,0.5)', overflowY: 'auto', background: 'rgba(13,17,23,0.5)' }}>
          {pkgs.map((pkg) => (
            <button
              key={pkg.id}
              onClick={() => setSelected(pkg.id)}
              style={{
                display: 'flex',
                flexDirection: 'column',
                gap: 4,
                width: '100%',
                padding: '10px 14px',
                background: selected === pkg.id ? 'rgba(88,166,255,0.1)' : 'transparent',
                border: 'none',
                borderBottom: '1px solid rgba(48,54,61,0.3)',
                borderLeft: selected === pkg.id ? '2px solid #58a6ff' : '2px solid transparent',
                cursor: 'pointer',
                textAlign: 'left',
                color: '#c9d1d9',
                transition: 'background 0.1s',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <span style={{ fontSize: 11, padding: '1px 6px', borderRadius: 4, background: `${pkgColor(pkg.type)}20`, color: pkgColor(pkg.type), fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.04em' }}>{pkg.type}</span>
                {pkg.status !== 'idle' && (
                  <span style={{ marginLeft: 'auto', fontSize: 10, color: pkg.status === 'installed' ? '#3fb950' : pkg.status === 'running' ? '#58a6ff' : pkg.status === 'installing' ? '#d29922' : '#f85149' }}>
                    {statusLabel(pkg.status)}
                  </span>
                )}
              </div>
              <span style={{ fontSize: 12, fontWeight: 600, color: '#e6edf3' }}>{pkg.name}</span>
              <span style={{ fontSize: 11, color: '#6e7681' }}>{pkg.size}</span>
            </button>
          ))}
        </div>

        {/* Detail + console */}
        <div style={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
          {sel ? (
            <div style={{ padding: '14px 16px', borderBottom: '1px solid rgba(48,54,61,0.4)', background: 'rgba(22,27,34,0.6)', flexShrink: 0 }}>
              <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 10 }}>
                <div>
                  <div style={{ fontSize: 15, fontWeight: 700, color: '#e6edf3', marginBottom: 4 }}>{sel.name}</div>
                  <div style={{ fontSize: 12, color: '#8b949e', marginBottom: 10 }}>{sel.description}</div>
                </div>
                {sel.status === 'installed' && <CheckCircle2 size={18} color="#3fb950" />}
              </div>
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                {sel.status === 'idle' && (
                  <button onClick={() => install(sel)} style={{ padding: '7px 16px', borderRadius: 7, border: 'none', background: '#238636', color: '#fff', fontSize: 12, fontWeight: 700, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6 }}>
                    <Package2 size={13} />
                    Install
                  </button>
                )}
                {sel.status === 'installed' && (
                  <>
                    <button onClick={() => run(sel)} style={{ padding: '7px 16px', borderRadius: 7, border: 'none', background: '#1f6feb', color: '#fff', fontSize: 12, fontWeight: 700, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6 }}>
                      <Play size={13} />
                      Run
                    </button>
                    <button onClick={() => remove(sel)} style={{ padding: '7px 16px', borderRadius: 7, border: '1px solid rgba(248,81,73,0.4)', background: 'rgba(248,81,73,0.1)', color: '#f85149', fontSize: 12, fontWeight: 700, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6 }}>
                      <Trash2 size={13} />
                      Remove
                    </button>
                  </>
                )}
                {sel.status === 'installing' && (
                  <span style={{ fontSize: 12, color: '#d29922', display: 'flex', alignItems: 'center', gap: 6 }}>
                    <span className="spin" style={{ display: 'inline-block' }}>⟳</span>
                    Installing...
                  </span>
                )}
                {sel.status === 'running' && (
                  <button onClick={() => { updatePkg(sel.id, { status: 'installed' }); log(`${sel.name} process terminated`); }} style={{ padding: '7px 16px', borderRadius: 7, border: '1px solid rgba(248,81,73,0.4)', background: 'rgba(248,81,73,0.1)', color: '#f85149', fontSize: 12, fontWeight: 700, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6 }}>
                    <X size={13} />
                    Kill
                  </button>
                )}
              </div>
            </div>
          ) : (
            <div style={{ padding: '24px 16px', color: '#484f58', fontSize: 13, textAlign: 'center' }}>Select a package to manage</div>
          )}

          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', padding: '10px 14px', gap: 6, overflow: 'hidden' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 11, color: '#6e7681', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em', flexShrink: 0 }}>
              <TerminalIcon size={11} />
              Console
            </div>
            <div
              ref={consoleRef}
              style={{ flex: 1, overflowY: 'auto', background: '#010409', borderRadius: 8, padding: '10px 12px', fontFamily: 'ui-monospace, monospace', fontSize: 11, lineHeight: 1.7, color: '#c9d1d9', border: '1px solid rgba(255,255,255,0.05)' }}
            >
              {lines.length === 0
                ? <span style={{ color: '#484f58' }}>No output yet. Install or run a package.</span>
                : lines.map((l, i) => (
                  <div key={i} style={{ color: l.startsWith('✓') ? '#3fb950' : l.startsWith('$') ? '#58a6ff' : l.includes('error') ? '#f85149' : '#c9d1d9' }}>{l}</div>
                ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
