'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { Minus, Maximize2, X } from 'lucide-react';
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

const TITLEBAR_H = 38;

export default function Window({ window: win, appComponents, onFocus, onClose, onMinimize, onMaximize, onMove, onResize }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const dragRef = useRef({ active: false, startX: 0, startY: 0, startWinX: 0, startWinY: 0 });
  const resizeRef = useRef({ active: false, startX: 0, startY: 0, startW: 0, startH: 0 });
  const listenersRef = useRef<{ move: ((ev: MouseEvent) => void) | null; up: (() => void) | null }>({ move: null, up: null });

  const [size, setSize] = useState({ w: win.width, h: win.height });
  const [pos, setPos] = useState({ x: win.x, y: win.y });

  useEffect(() => {
    setSize({ w: win.width, h: win.height });
    setPos({ x: win.x, y: win.y });
  }, [win.width, win.height, win.x, win.y]);

  useEffect(() => {
    return () => {
      dragRef.current.active = false;
      resizeRef.current.active = false;
      if (listenersRef.current.move) document.removeEventListener('mousemove', listenersRef.current.move);
      if (listenersRef.current.up) document.removeEventListener('mouseup', listenersRef.current.up);
      listenersRef.current = { move: null, up: null };
    };
  }, []);

  const handleTitleBarMouseDown = useCallback((e: React.MouseEvent) => {
    if ((e.target as HTMLElement).closest('button')) return;
    e.preventDefault();
    onFocus();
    dragRef.current = { active: true, startX: e.clientX, startY: e.clientY, startWinX: pos.x, startWinY: pos.y };

    const handleMove = (ev: MouseEvent) => {
      if (!dragRef.current.active) return;
      const newX = Math.max(0, dragRef.current.startWinX + ev.clientX - dragRef.current.startX);
      const newY = Math.max(0, dragRef.current.startWinY + ev.clientY - dragRef.current.startY);
      setPos({ x: newX, y: newY });
      onMove(newX, newY);
    };
    const handleUp = () => {
      dragRef.current.active = false;
      document.removeEventListener('mousemove', handleMove);
      document.removeEventListener('mouseup', handleUp);
      listenersRef.current = { move: null, up: null };
    };
    listenersRef.current = { move: handleMove, up: handleUp };
    document.addEventListener('mousemove', handleMove);
    document.addEventListener('mouseup', handleUp);
  }, [pos, onFocus, onMove]);

  const handleResizeMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    resizeRef.current = { active: true, startX: e.clientX, startY: e.clientY, startW: size.w, startH: size.h };

    const handleMove = (ev: MouseEvent) => {
      if (!resizeRef.current.active) return;
      const newW = Math.max(380, resizeRef.current.startW + ev.clientX - resizeRef.current.startX);
      const newH = Math.max(200, resizeRef.current.startH + ev.clientY - resizeRef.current.startY);
      setSize({ w: newW, h: newH });
      onResize(newW, newH);
    };
    const handleUp = () => {
      resizeRef.current.active = false;
      document.removeEventListener('mousemove', handleMove);
      document.removeEventListener('mouseup', handleUp);
      listenersRef.current = { move: null, up: null };
    };
    listenersRef.current = { move: handleMove, up: handleUp };
    document.addEventListener('mousemove', handleMove);
    document.addEventListener('mouseup', handleUp);
  }, [size, onResize]);

  const AppComponent = appComponents[win.app];
  const isMaximized = win.maximized;

  return (
    <div
      ref={ref}
      onClick={onFocus}
      style={{
        position: 'absolute',
        left: isMaximized ? 0 : pos.x,
        top: isMaximized ? 0 : pos.y,
        width: isMaximized ? '100%' : size.w,
        height: isMaximized ? 'calc(100vh - 46px)' : size.h,
        zIndex: win.zIndex,
        border: `1px solid ${win.focused ? 'rgba(88,166,255,0.25)' : 'rgba(48,54,61,0.6)'}`,
        borderRadius: isMaximized ? 0 : 10,
        overflow: 'hidden',
        display: 'flex',
        flexDirection: 'column',
        boxShadow: win.focused
          ? '0 16px 48px rgba(0,0,0,0.6), 0 0 0 1px rgba(88,166,255,0.1)'
          : '0 4px 16px rgba(0,0,0,0.4)',
        transition: 'box-shadow 0.15s, border-color 0.15s',
      }}
    >
      {/* Title bar */}
      <div
        onMouseDown={handleTitleBarMouseDown}
        style={{
          height: TITLEBAR_H,
          background: win.focused ? 'rgba(22,27,34,0.98)' : 'rgba(13,17,23,0.95)',
          borderBottom: `1px solid ${win.focused ? 'rgba(88,166,255,0.15)' : 'rgba(48,54,61,0.5)'}`,
          display: 'flex',
          alignItems: 'center',
          padding: '0 10px',
          cursor: 'move',
          userSelect: 'none',
          gap: 8,
        }}
      >
        {/* Traffic lights */}
        <div style={{ display: 'flex', gap: 6, flexShrink: 0 }}>
          <button
            onClick={(e) => { e.stopPropagation(); onClose(); }}
            style={trafficLight('#f85149')}
            title="Close"
            aria-label={`Close ${win.title}`}
          >
            <X size={8} strokeWidth={2.5} />
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); onMinimize(); }}
            style={trafficLight('#d29922')}
            title="Minimize"
            aria-label={`Minimize ${win.title}`}
          >
            <Minus size={8} strokeWidth={2.5} />
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); onMaximize(); }}
            style={trafficLight('#238636')}
            title={win.maximized ? 'Restore' : 'Maximize'}
            aria-label={win.maximized ? `Restore ${win.title}` : `Maximize ${win.title}`}
          >
            <Maximize2 size={7} strokeWidth={2.5} />
          </button>
        </div>

        {/* Title */}
        <span style={{
          flex: 1,
          fontSize: 12,
          fontWeight: 600,
          color: win.focused ? '#c9d1d9' : '#6e7681',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
          textAlign: 'center',
          paddingRight: 52,
        }}>
          {win.title}
        </span>
      </div>

      {/* App content */}
      <div style={{ flex: 1, overflow: 'auto', background: '#0d0d1a', color: '#ccc', fontSize: 13 }}>
        {AppComponent ? <AppComponent /> : (
          <div style={{ padding: 24, color: '#484f58', textAlign: 'center', paddingTop: 40 }}>
            App not found: <code style={{ color: '#58a6ff', fontSize: 11 }}>{win.app}</code>
          </div>
        )}
      </div>

      {/* Resize handle */}
      {!isMaximized && (
        <div
          onMouseDown={handleResizeMouseDown}
          style={{
            position: 'absolute',
            bottom: 0,
            right: 0,
            width: 16,
            height: 16,
            cursor: 'nwse-resize',
            zIndex: 1,
          }}
        />
      )}
    </div>
  );
}

function trafficLight(bg: string): React.CSSProperties {
  return {
    width: 13,
    height: 13,
    borderRadius: '50%',
    background: bg,
    border: 'none',
    cursor: 'pointer',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    color: 'rgba(0,0,0,0.5)',
    padding: 0,
    flexShrink: 0,
  };
}
