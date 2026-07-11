'use client';

import { useCallback, useEffect, useState } from 'react';
import { Folder, FileText } from 'lucide-react';
import { apiFetch, getTokens, getGatewayUrl } from '../../boot/BootSequence';

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
  const [quota, setQuota] = useState({ used: 0, total: 10737418240 });
  const [notification, setNotification] = useState<{type: 'success'|'error', message: string}|null>(null);

  const listFiles = useCallback(async (path: string) => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ path });
      const data = await apiFetch(`/storage/list?${params}`);
      setFiles(data.files || []);
    } catch (err) {
      console.error('list files error:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  const uploadFile = useCallback(async (path: string, file: File) => {
    try {
      setLoading(true);
      const params = new URLSearchParams({ path });
      const token = getTokens()?.accessToken;
      if (!token) throw new Error('Not authenticated');
      const resp = await fetch(`${getGatewayUrl()}/storage/upload?${params}`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` },
        body: file,
      });
      if (!resp.ok) {
        const text = await resp.text().catch(() => 'unknown');
        throw new Error(`Upload failed (${resp.status}): ${text}`);
      }
      setNotification({ type: 'success', message: 'Uploaded' });
      listFiles(currentPath);
    } catch (e: any) {
      setNotification({ type: 'error', message: e.message || 'Upload failed' });
    } finally {
      setLoading(false);
    }
  }, [currentPath, listFiles]);

  const deleteFile = useCallback(async (filePath: string) => {
    try {
      const params = new URLSearchParams({ path: filePath });
      await apiFetch(`/storage/delete?${params}`, { method: 'DELETE' });
      listFiles(currentPath);
    } catch (err) {
      console.error('delete error:', err);
    }
  }, [currentPath, listFiles]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const profile = await apiFetch('/auth/profile');
        if (!cancelled) {
          setQuota({ used: profile.storageUsed, total: profile.storageQuota });
        }
      } catch {
        if (!cancelled) {
          setQuota({ used: 0, total: 10737418240 });
        }
      }
      try {
        await listFiles('');
      } catch {
        if (!cancelled) {
          setFiles([]);
        }
      }
    })();
    return () => { cancelled = true; };
  }, [listFiles]);

  const storagePercent = quota.total > 0 ? (quota.used / quota.total) * 100 : 0;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', background: '#0d1117', color: '#ccc', fontFamily: 'system-ui, sans-serif' }}>
      <div style={{ position: 'relative', display: 'flex', alignItems: 'center', gap: 8, padding: '8px 12px', background: '#161b22', borderBottom: '1px solid #30363d' }}>
        <span style={{ fontSize: 13, color: '#888' }}>Drive</span>
        <span style={{ fontSize: 12, color: '#444' }}>/</span>
        <span style={{ fontSize: 13 }}>{currentPath || '~'}</span>
        <div style={{ flex: 1 }} />
        <label style={{ padding: '4px 12px', background: '#238636', color: '#fff', borderRadius: 4, cursor: 'pointer', fontSize: 12 }}>
          Upload
          <input type="file" hidden onChange={(e) => { const f = e.target.files?.[0]; if (f) uploadFile('', f); }} />
        </label>
        {notification && (
          <div style={{
            position: 'absolute', top: 8, right: 8, padding: '6px 12px', borderRadius: 4, fontSize: 12,
            background: notification.type === 'success' ? '#238636' : '#f85149', color: '#fff', zIndex: 10,
          }}>{notification.message}</div>
        )}
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
            <span style={{ marginRight: 8 }}>{file.isDir ? <Folder size={14} color="#58a6ff" /> : <FileText size={14} color="#8b949e" />}</span>
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
