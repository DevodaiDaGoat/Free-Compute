'use client';

import { useCallback, useEffect, useState } from 'react';
import { getGatewayUrl, getTokens } from '../../boot/BootSequence';

interface FileItem {
  id: string;
  name: string;
  path: string;
  size: number;
  mimeType: string;
  isDir: boolean;
  updatedAt: string;
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

export default function FilesApp() {
  const [files, setFiles] = useState<FileItem[]>([]);
  const [currentPath, setCurrentPath] = useState('');
  const [loading, setLoading] = useState(false);
  const [quota, setQuota] = useState({ used: 0, total: 107374182400 });

  const listFiles = useCallback(async (path: string) => {
    setLoading(true);
    try {
      const tokens = getTokens();
      if (!tokens) return;
      const params = new URLSearchParams({ userId: tokens.accessToken, path });
      const res = await fetch(`${getGatewayUrl()}/storage/list?${params}`);
      if (!res.ok) throw new Error('Failed to list files');
      const data = await res.json();
      setFiles(data.files || []);
    } catch (err) {
      console.error('list files error:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  const uploadFile = useCallback(async (file: File) => {
    try {
      const tokens = getTokens();
      if (!tokens) return;
      const path = currentPath ? `${currentPath}/${file.name}` : file.name;
      const params = new URLSearchParams({ userId: tokens.accessToken, path });
      const res = await fetch(`${getGatewayUrl()}/storage/upload?${params}`, {
        method: 'POST',
        body: file,
      });
      if (!res.ok) throw new Error('Upload failed');
      listFiles(currentPath);
    } catch (err) {
      console.error('upload error:', err);
    }
  }, [currentPath, listFiles]);

  const deleteFile = useCallback(async (filePath: string) => {
    try {
      const tokens = getTokens();
      if (!tokens) return;
      const params = new URLSearchParams({ userId: tokens.accessToken, path: filePath });
      const res = await fetch(`${getGatewayUrl()}/storage/delete?${params}`, { method: 'DELETE' });
      if (!res.ok) throw new Error('Delete failed');
      listFiles(currentPath);
    } catch (err) {
      console.error('delete error:', err);
    }
  }, [currentPath, listFiles]);

  useEffect(() => { listFiles(''); }, []);

  const storagePercent = quota.total > 0 ? (quota.used / quota.total) * 100 : 0;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', background: '#0d1117', color: '#ccc', fontFamily: 'system-ui, sans-serif' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '8px 12px', background: '#161b22', borderBottom: '1px solid #30363d' }}>
        <span style={{ fontSize: 13, color: '#888' }}>Drive</span>
        <span style={{ fontSize: 12, color: '#444' }}>/</span>
        <span style={{ fontSize: 13 }}>{currentPath || '~'}</span>
        <div style={{ flex: 1 }} />
        <label style={{ padding: '4px 12px', background: '#238636', color: '#fff', borderRadius: 4, cursor: 'pointer', fontSize: 12 }}>
          Upload
          <input type="file" hidden onChange={(e) => { const f = e.target.files?.[0]; if (f) uploadFile(f); }} />
        </label>
      </div>

      <div style={{ flex: 1, overflow: 'auto' }}>
        {currentPath && (
          <div
            onClick={() => setCurrentPath(currentPath.split('/').slice(0, -1).join('/'))}
            style={{ padding: '8px 16px', cursor: 'pointer', borderBottom: '1px solid #21262d', color: '#58a6ff', fontSize: 13 }}
          >..</div>
        )}
        {files.map((file) => (
          <div key={file.id} style={{ display: 'flex', alignItems: 'center', padding: '8px 16px', borderBottom: '1px solid #21262d', fontSize: 13 }}>
            <span style={{ marginRight: 8 }}>{file.isDir ? '📁' : '📄'}</span>
            <span
              onClick={() => file.isDir && setCurrentPath(file.path)}
              style={{ cursor: file.isDir ? 'pointer' : 'default', flex: 1, color: file.isDir ? '#58a6ff' : '#ccc' }}
            >{file.name}</span>
            <span style={{ color: '#888', width: 80, textAlign: 'right' }}>{file.isDir ? '-' : formatSize(file.size)}</span>
            <span style={{ color: '#888', width: 160, textAlign: 'right', fontSize: 11 }}>{file.updatedAt}</span>
            <button
              onClick={() => deleteFile(file.path)}
              style={{ background: 'none', border: 'none', color: '#f44', cursor: 'pointer', marginLeft: 8, fontSize: 12 }}
            >Delete</button>
          </div>
        ))}
        {!loading && files.length === 0 && (
          <div style={{ padding: 32, textAlign: 'center', color: '#555' }}>No files. Upload something!</div>
        )}
      </div>

      <div style={{ padding: '6px 12px', borderTop: '1px solid #21262d', display: 'flex', alignItems: 'center', gap: 8, fontSize: 11, color: '#888' }}>
        <div style={{ flex: 1, height: 4, background: '#21262d', borderRadius: 2 }}>
          <div style={{ width: `${storagePercent}%`, height: '100%', background: '#238636', borderRadius: 2 }} />
        </div>
        <span>{formatSize(quota.used)} / {formatSize(quota.total)}</span>
      </div>
    </div>
  );
}
