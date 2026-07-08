'use client';

import type { AppWindow } from '../system/types';
import type { ReactNode } from 'react';
import Window from './Window';

interface Props {
  windows: AppWindow[];
  appComponents: Record<string, () => ReactNode>;
  onFocus: (id: string) => void;
  onClose: (id: string) => void;
  onMinimize: (id: string) => void;
  onMaximize: (id: string) => void;
  onMove: (id: string, x: number, y: number) => void;
  onResize: (id: string, w: number, h: number) => void;
}

export default function WindowManager({ windows, appComponents, onFocus, onClose, onMinimize, onMaximize, onMove, onResize }: Props) {
  const visibleWindows = windows.filter((w) => !w.minimized);

  return (
    <>
      {visibleWindows.map((win) => (
        <Window
          key={win.id}
          window={win}
          appComponents={appComponents}
          onFocus={() => onFocus(win.id)}
          onClose={() => onClose(win.id)}
          onMinimize={() => onMinimize(win.id)}
          onMaximize={() => onMaximize(win.id)}
          onMove={(x, y) => onMove(win.id, x, y)}
          onResize={(w, h) => onResize(win.id, w, h)}
        />
      ))}
    </>
  );
}
