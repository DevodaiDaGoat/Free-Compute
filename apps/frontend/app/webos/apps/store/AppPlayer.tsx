'use client';

import React, { useState, useRef } from 'react';
import { Package2, FileCode, Monitor, Play, Trash2, FolderOpen, FileInput, X } from 'lucide-react';

interface FileItem {
  id: string;
  name: string;
  type: 'deb' | 'exe' | 'unknown';
  size: string;
  installedDate: string;
  description: string;
}

const initialFiles: FileItem[] = [
  { id: '1', name: 'nano_7.2-1_amd64.deb', type: 'deb', size: '245 KB', installedDate: '2026-06-15', description: 'GNU nano text editor for command-line editing' },
  { id: '2', name: 'htop_3.3.0-1_amd64.deb', type: 'deb', size: '128 KB', installedDate: '2026-06-10', description: 'Interactive process viewer for Linux' },
  { id: '3', name: 'node-v20.11.0-win-x64.exe', type: 'exe', size: '31.2 MB', installedDate: '2026-05-22', description: 'Node.js JavaScript runtime for Windows' },
  { id: '4', name: 'git-2.43.0-64.exe', type: 'exe', size: '52.1 MB', installedDate: '2026-05-20', description: 'Distributed version control system' },
];

export default function AppPlayerApp() {
  const [files, setFiles] = useState<FileItem[]>(initialFiles);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [consoleLog, setConsoleLog] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [activeProcess, setActiveProcess] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const selectedFile = files.find(f => f.id === selectedId);

  const addLog = (line: string) => {
    setConsoleLog(prev => [...prev, line]);
  };

  const handleInstall = (file: FileItem) => {
    if (loading) return;
    setLoading(true);
    setActiveProcess(file.id);
    addLog(`$ dpkg -i ${file.name}`);
    setTimeout(() => {
      addLog(`Selecting previously unselected package ${file.name.replace('.deb', '')}.`);
      addLog(`(Reading database ... 123456 files and directories currently installed.)`);
      addLog(`Preparing to unpack .../${file.name} ...`);
      addLog(`Unpacking ${file.name.replace('.deb', '')} ...`);
      addLog(`Setting up ${file.name.replace('.deb', '')} ...`);
      addLog(`Processing triggers for man-db (2.11.2) ...`);
      addLog(`Installed successfully.`);
      setLoading(false);
      setActiveProcess(null);
    }, 1200);
  };

  const handleRun = (file: FileItem) => {
    if (loading) return;
    setLoading(true);
    setActiveProcess(file.id);
    addLog(`$ ./${file.name}`);
    setTimeout(() => {
      addLog(`Loading dependencies... OK`);
      addLog(`Initializing runtime... OK`);
      addLog(`Process started (PID 12345)`);
      addLog(`Memory: 0.4 MB | Threads: 2`);
      setLoading(false);
      setActiveProcess(null);
    }, 2000);
  };

  const handleUninstall = (file: FileItem) => {
    if (file.type === 'deb') {
      addLog(`$ dpkg -r ${file.name.replace('.deb', '')}`);
      addLog(`(Reading database ... 123456 files and directories currently installed.)`);
      addLog(`Removing ${file.name.replace('.deb', '')} ...`);
      addLog(`Purging configuration files for ${file.name.replace('.deb', '')} ...`);
      addLog(`Uninstalled successfully.`);
    } else {
      addLog(`$ rm /Applications/${file.name}`);
      addLog(`File removed.`);
    }
    setFiles(prev => prev.filter(f => f.id !== file.id));
    if (selectedId === file.id) setSelectedId(null);
  };

  const handleOpenWith = (file: FileItem) => {
    addLog(`$ xdg-mime query default application/${file.type === 'deb' ? 'x-deb' : 'vnd.microsoft.portable-executable'}`);
    addLog(`Available handlers: default, vscode, archive-manager`);
    addLog(`Opening with default handler...`);
  };

  const handleUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const uploaded = e.target.files?.[0];
    if (!uploaded) return;

    const name = uploaded.name;
    let type: FileItem['type'] = 'unknown';
    if (name.endsWith('.deb')) type = 'deb';
    else if (name.endsWith('.exe')) type = 'exe';

    const newFile: FileItem = {
      id: Date.now().toString(),
      name,
      type,
      size: (uploaded.size / 1024).toFixed(0) + ' KB',
      installedDate: new Date().toISOString().split('T')[0],
      description: type === 'deb' ? 'Debian software package' : type === 'exe' ? 'Portable executable' : 'Unknown executable format',
    };

    setFiles(prev => [...prev, newFile]);
    setSelectedId(newFile.id);
    addLog(`Importing ${name} ...`);
    addLog(`Format detected: ${type.toUpperCase()}`);
    addLog(`Ready to install.`);
  };

  const clearConsole = () => setConsoleLog([]);

  const iconColor = selectedFile?.type === 'deb' ? '#fbbf24' : selectedFile?.type === 'exe' ? '#38bdf8' : '#888';
  const IconComponent = selectedFile?.type === 'deb' ? FileCode : selectedFile?.type === 'exe' ? Monitor : Package2;

  return (
    <div style={{ display: 'flex', height: '100%', background: '#0d1117', color: '#c9d1d9', fontFamily: 'system-ui, -apple-system, sans-serif' }}>
      <input
        ref={fileInputRef}
        type="file"
        accept=".deb,.exe"
        onChange={handleUpload}
        style={{ display: 'none' }}
      />

      <style>{`
        @keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
        @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.5; } }
        .spin { animation: spin 1s linear infinite; }
        .pulse { animation: pulse 1.5s ease-in-out infinite; }
      `}</style>

      <div style={{ width: 220, background: '#0b0f19', borderRight: '1px solid #1e293b', display: 'flex', flexDirection: 'column', flexShrink: 0 }}>
        <div style={{ padding: 12, borderBottom: '1px solid #1e293b', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontWeight: 600, fontSize: 13 }}>
            <Package2 size={16} />
            App Player
          </div>
          <button
            onClick={() => fileInputRef.current?.click()}
            style={{
              background: 'rgba(56,189,248,0.1)',
              border: '1px solid rgba(56,189,248,0.3)',
              color: '#38bdf8',
              borderRadius: 4,
              padding: '2px 8px',
              fontSize: 11,
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: 4,
            }}
            title="Import .deb or .exe"
          >
            <FileInput size={14} />
            Import
          </button>
        </div>

        <div style={{ flex: 1, overflowY: 'auto' }}>
          {files.map((file) => (
            <div
              key={file.id}
              onClick={() => { setSelectedId(file.id); addLog(`Selected: ${file.name}`); }}
              style={{
                padding: '10px 12px',
                cursor: 'pointer',
                borderBottom: '1px solid #0f172a',
                background: selectedId === file.id ? 'rgba(56,189,248,0.1)' : 'transparent',
                borderLeft: selectedId === file.id ? '2px solid #18e2ff' : '2px solid transparent',
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                transition: 'background 0.1s ease',
              }}
            >
              <div style={{ color: file.type === 'deb' ? '#fbbf24' : '#38bdf8', display: 'flex' }}>
                {file.type === 'deb' ? <FileCode size={16} /> : <Monitor size={16} />}
              </div>
              <div style={{ overflow: 'hidden' }}>
                <div style={{ fontSize: 12, color: '#eee', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {file.name}
                </div>
                <div style={{ fontSize: 10, color: '#666' }}>
                  {file.size} | {file.type.toUpperCase()}
                </div>
              </div>
            </div>
          ))}

          {files.length === 0 && (
            <div style={{ padding: 20, textAlign: 'center', color: '#555', fontSize: 12 }}>
              No files imported.<br />Click Import to add .deb/.exe.
            </div>
          )}
        </div>
      </div>

      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', minWidth: 0 }}>
        {selectedFile ? (
          <>
            <div style={{ padding: 20, borderBottom: '1px solid #1e293b', overflowY: 'auto' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12 }}>
                <div style={{
                  width: 40, height: 40, borderRadius: 8,
                  background: selectedFile.type === 'deb' ? 'rgba(251,191,36,0.1)' : 'rgba(56,189,248,0.1)',
                  border: '1px solid ' + (selectedFile.type === 'deb' ? 'rgba(251,191,36,0.2)' : 'rgba(56,189,248,0.2)'),
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                }}>
                  <IconComponent size={24} color={iconColor} />
                </div>
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontSize: 16, fontWeight: 600, color: '#eee', wordBreak: 'break-all' }}>{selectedFile.name}</div>
                  <div style={{ fontSize: 12, color: '#888' }}>{selectedFile.description}</div>
                </div>
              </div>

              <div style={{ display: 'flex', gap: 8, marginBottom: 16, flexWrap: 'wrap' }}>
                <div style={{ fontSize: 11, color: '#666', background: '#0b0f19', padding: '4px 8px', borderRadius: 4, border: '1px solid #1e293b' }}>
                  Type: {selectedFile.type.toUpperCase()}
                </div>
                <div style={{ fontSize: 11, color: '#666', background: '#0b0f19', padding: '4px 8px', borderRadius: 4, border: '1px solid #1e293b' }}>
                  Size: {selectedFile.size}
                </div>
                <div style={{ fontSize: 11, color: '#666', background: '#0b0f19', padding: '4px 8px', borderRadius: 4, border: '1px solid #1e293b' }}>
                  Installed: {selectedFile.installedDate}
                </div>
              </div>

              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                {selectedFile.type === 'deb' && (
                  <button
                    onClick={() => handleInstall(selectedFile)}
                    disabled={loading}
                    style={{
                      background: 'rgba(24,226,255,0.1)',
                      border: '1px solid rgba(24,226,255,0.4)',
                      color: '#18e2ff',
                      padding: '6px 12px',
                      borderRadius: 4,
                      cursor: loading ? 'not-allowed' : 'pointer',
                      fontSize: 12,
                      fontWeight: 500,
                      display: 'flex',
                      alignItems: 'center',
                      gap: 6,
                      opacity: loading ? 0.6 : 1,
                    }}
                  >
                    <Play size={14} /> Install
                  </button>
                )}
                {selectedFile.type === 'exe' && (
                  <button
                    onClick={() => handleRun(selectedFile)}
                    disabled={loading}
                    style={{
                      background: 'rgba(24,226,255,0.1)',
                      border: '1px solid rgba(24,226,255,0.4)',
                      color: '#18e2ff',
                      padding: '6px 12px',
                      borderRadius: 4,
                      cursor: loading ? 'not-allowed' : 'pointer',
                      fontSize: 12,
                      fontWeight: 500,
                      display: 'flex',
                      alignItems: 'center',
                      gap: 6,
                      opacity: loading ? 0.6 : 1,
                    }}
                  >
                    <Play size={14} /> {loading ? 'Running...' : 'Run'}
                  </button>
                )}
                <button
                  onClick={() => handleUninstall(selectedFile)}
                  disabled={loading}
                  style={{
                    background: 'rgba(248,81,73,0.1)',
                    border: '1px solid rgba(248,81,73,0.4)',
                    color: '#f85149',
                    padding: '6px 12px',
                    borderRadius: 4,
                    cursor: loading ? 'not-allowed' : 'pointer',
                    fontSize: 12,
                    fontWeight: 500,
                    display: 'flex',
                    alignItems: 'center',
                    gap: 6,
                    opacity: loading ? 0.6 : 1,
                  }}
                >
                  <Trash2 size={14} /> Uninstall
                </button>
                <button
                  onClick={() => handleOpenWith(selectedFile)}
                  disabled={loading}
                  style={{
                    background: 'rgba(255,255,255,0.03)',
                    border: '1px solid #2a2a4a',
                    color: '#aaa',
                    padding: '6px 12px',
                    borderRadius: 4,
                    cursor: loading ? 'not-allowed' : 'pointer',
                    fontSize: 12,
                    fontWeight: 500,
                    display: 'flex',
                    alignItems: 'center',
                    gap: 6,
                    opacity: loading ? 0.6 : 1,
                  }}
                >
                  <FolderOpen size={14} /> Open With
                </button>
              </div>
            </div>

            <div style={{ flex: 1, display: 'flex', flexDirection: 'column', background: '#080c14', minHeight: 0 }}>
              <div style={{
                padding: '6px 12px',
                background: '#0b0f19',
                borderBottom: '1px solid #1e293b',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
              }}>
                <span style={{ fontSize: 11, color: '#666', fontWeight: 600, letterSpacing: '0.5px' }}>TERMINAL OUTPUT</span>
                <button
                  onClick={clearConsole}
                  style={{ background: 'none', border: 'none', color: '#555', cursor: 'pointer', fontSize: 10, display: 'flex', alignItems: 'center', gap: 4 }}
                >
                  <X size={12} /> Clear
                </button>
              </div>
              <div style={{ flex: 1, padding: 12, fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace', fontSize: 12, overflowY: 'auto', color: '#8b949e' }}>
                {consoleLog.length === 0 && (
                  <span style={{ color: '#333' }}>// No output yet. Select an action to interact with the package.</span>
                )}
                {consoleLog.map((line, i) => (
                  <div key={i} style={{ marginBottom: 2, color: line.startsWith('$') ? '#58a6ff' : line.startsWith('Installed') || line.startsWith('Uninstalled') || line.startsWith('File removed.') ? '#3fb950' : '#c9d1d9' }}>
                    {line}
                  </div>
                ))}
                {loading && (
                  <div style={{ marginTop: 8, display: 'flex', alignItems: 'center', gap: 8, color: '#58a6ff' }}>
                    <svg className="spin" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M21 12a9 9 0 11-6.219-8.56" />
                    </svg>
                    Processing...
                  </div>
                )}
                {activeProcess && !loading && (
                  <div style={{ marginTop: 8, color: '#3fb950', display: 'flex', alignItems: 'center', gap: 6 }}>
                    <Monitor size={14} /> Process active (PID 12345)
                  </div>
                )}
              </div>
            </div>
          </>
        ) : (
          <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#333', flexDirection: 'column', gap: 12 }}>
            <Package2 size={48} style={{ opacity: 0.3 }} />
            <div style={{ fontSize: 14 }}>Select a file to view details</div>
          </div>
        )}
      </div>
    </div>
  );
}
