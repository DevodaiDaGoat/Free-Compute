'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { Minimize2, Maximize2, X } from 'lucide-react';
import type { AppWindow } from '../system/types';
import type { ReactNode } from 'react';

interface Props {
  window: AppWindow;
  appComponents: Record<string, () => ReactNode>;
  onFocus: () => void;
  onClose: () => void;
  onMinimize: () => void;
  onMaximize: () => void;
  onMove: (x: number, y: number) => void;
  onResize: (w: number, h: number) => void;
}

const titleBarH = 36;

export default function Window({ window: win, appComponents, onFocus, onClose, onMinimize, onMaximize, onMove, onResize }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const drag = useRef({ dragging: false, startX: 0, startY: 0, startWinX: 0, startWinY: 0 });
  const resize = useRef({ resizing: false, startX: 0, startY: 0, startW: 0, startH: 0, dir: '' });
  const [size, setSize] = useState({ w: win.width, h: win.height });
  const [pos, setPos] = useState({ x: win.x, y: win.y });

  useEffect(() => {
    setSize({ w: win.width, h: win.height });
    setPos({ x: win.x, y: win.y });
  }, [win.width, win.height, win.x, win.y]);

  const onMouseDown = useCallback((e: React.MouseEvent) => {
    onFocus();
    drag.current = { dragging: true, startX: e.clientX, startY: e.clientY, startWinX: pos.x, startWinY: pos.y };
    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  }, [pos, onFocus]);

  const onMouseMove = useCallback((e: MouseEvent) => {
    if (!drag.current.dragging) return;
    const dx = e.clientX - drag.current.startX;
    const dy = e.clientY - drag.current.startY;
    const newX = Math.max(0, drag.current.startWinX + dx);
    const newY = Math.max(0, drag.current.startWinY + dy);
    setPos({ x: newX, y: newY });
    onMove(newX, newY);
  }, [onMove]);

  const onMouseUp = useCallback(() => {
    drag.current.dragging = false;
    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', onMouseUp);
  }, []);

  const onResizeStart = useCallback((dir: string) => (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    resize.current = { resizing: true, startX: e.clientX, startY: e.clientY, startW: size.w, startH: size.h, dir };
    document.addEventListener('mousemove', onResizeMove);
    document.addEventListener('mouseup', onResizeEnd);
  }, [size]);

  const onResizeMove = useCallback((e: MouseEvent) => {
    if (!resize.current.resizing) return;
    const dx = e.clientX - resize.current.startX;
    const dy = e.clientY - resize.current.startY;
    let newW = size.w;
    let newH = size.h;
    if (resize.current.dir.includes('e')) newW = Math.max(400, resize.current.startW + dx);
    if (resize.current.dir.includes('s')) newH = Math.max(200, resize.current.startH + dy);
    setSize({ w: newW, h: newH });
    onResize(newW, newH);
  }, [size, onResize]);

  const onResizeEnd = useCallback(() => {
    resize.current.resizing = false;
    document.removeEventListener('mousemove', onResizeMove);
    document.removeEventListener('mouseup', onResizeEnd);
  }, []);

  const AppComponent = appComponents[win.app];
  const isMaximized = win.maximized;

  return (
    <div
      ref={ref}
      style={{
        position: 'absolute',
        left: isMaximized ? 0 : pos.x,
        top: isMaximized ? 0 : pos.y,
        width: isMaximized ? '100%' : size.w,
        height: isMaximized ? 'calc(100vh - 44px)' : size.h,
        zIndex: win.zIndex,
        border: '1px solid #2a2a4a',
        borderRadius: 8,
        overflow: 'hidden',
        display: 'flex',
        flexDirection: 'column',
        boxShadow: win.focused ? '0 8px 32px rgba(0,0,0,0.5)' : '0 2px 8px rgba(0,0,0,0.3)',
      }}
    >
      <div
        onMouseDown={onMouseDown}
        style={{
          height: titleBarH,
          background: win.focused ? '#1a1a3e' : '#111128',
          display: 'flex',
          alignItems: 'center',
          padding: '0 8px',
          cursor: 'move',
          userSelect: 'none',
        }}
      >
        <span style={{ flex: 1, fontSize: 12, color: '#aaa', marginLeft: 8 }}>{win.title}</span>
        <button onClick={onMinimize} style={btnStyle} title="Minimize"><Minimize2 size={12} /></button>
        <button onClick={onMaximize} style={btnStyle} title={win.maximized ? 'Restore' : 'Maximize'}>{win.maximized ? <Maximize2 size={12} style={{ transform: 'rotate(180deg)' }} /> : <Maximize2 size={12} />}</button>
        <button onClick={onClose} style={{ ...btnStyle, color: '#f44' }}><X size={12} /></button>
      </div>
      <div style={{ flex: 1, overflow: 'auto', background: '#0d0d1a', color: '#ccc', fontSize: 13 }}>
        {AppComponent ? <AppComponent /> : <div style={{ padding: 24, color: '#666' }}>App content</div>}
      </div>
      <div style={{ position: 'absolute', bottom: 0, right: 0, width: 12, height: 12, cursor: 'nwse-resize' }} onMouseDown={onResizeStart('se')} />
    </div>
  );
}

const btnStyle: React.CSSProperties = {
  background: 'none', border: 'none', color: '#aaa', cursor: 'pointer',
  width: 30, height: 30, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 12,
};
