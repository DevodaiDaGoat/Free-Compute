import { create } from "zustand";

import {
  DEFAULT_WINDOW_SIZE,
  MIN_WINDOW_SIZE,
  WINDOW_CASCADE_OFFSET,
  Z_INDEX_BASE,
} from "@/lib/constants";
import type { DesktopWindow, Point, Size } from "@/lib/types";
import { clamp, generateId } from "@/lib/utils";

export interface OpenWindowOptions {
  app: string;
  title: string;
  width?: number;
  height?: number;
  x?: number;
  y?: number;
  closable?: boolean;
  resizable?: boolean;
}

interface WindowState {
  windows: DesktopWindow[];
  topZIndex: number;
  openWindow: (options: OpenWindowOptions) => string;
  closeWindow: (id: string) => void;
  focusWindow: (id: string) => void;
  minimizeWindow: (id: string) => void;
  toggleMaximize: (id: string) => void;
  restoreWindow: (id: string) => void;
  moveWindow: (id: string, position: Point) => void;
  resizeWindow: (id: string, size: Size) => void;
}

function nextZIndex(state: WindowState): number {
  return state.topZIndex + 1;
}

export const useWindowStore = create<WindowState>((set, get) => ({
  windows: [],
  topZIndex: Z_INDEX_BASE,

  openWindow: (options) => {
    const id = generateId("win");
    set((state) => {
      const zIndex = nextZIndex(state);
      const count = state.windows.length;
      const width = options.width ?? DEFAULT_WINDOW_SIZE.width;
      const height = options.height ?? DEFAULT_WINDOW_SIZE.height;
      const offset = (count % 6) * WINDOW_CASCADE_OFFSET;
      const win: DesktopWindow = {
        id,
        title: options.title,
        app: options.app,
        x: options.x ?? 80 + offset,
        y: options.y ?? 60 + offset,
        width,
        height,
        zIndex,
        minimized: false,
        maximized: false,
        focused: true,
        closable: options.closable ?? true,
        resizable: options.resizable ?? true,
      };
      return {
        topZIndex: zIndex,
        windows: [
          ...state.windows.map((w) => ({ ...w, focused: false })),
          win,
        ],
      };
    });
    return id;
  },

  closeWindow: (id) => {
    set((state) => ({
      windows: state.windows.filter((w) => w.id !== id),
    }));
  },

  focusWindow: (id) => {
    set((state) => {
      const target = state.windows.find((w) => w.id === id);
      if (!target) return state;
      const zIndex = nextZIndex(state);
      return {
        topZIndex: zIndex,
        windows: state.windows.map((w) =>
          w.id === id
            ? { ...w, focused: true, minimized: false, zIndex }
            : { ...w, focused: false },
        ),
      };
    });
  },

  minimizeWindow: (id) => {
    set((state) => ({
      windows: state.windows.map((w) =>
        w.id === id ? { ...w, minimized: true, focused: false } : w,
      ),
    }));
  },

  toggleMaximize: (id) => {
    set((state) => ({
      windows: state.windows.map((w) =>
        w.id === id ? { ...w, maximized: !w.maximized } : w,
      ),
    }));
  },

  restoreWindow: (id) => {
    get().focusWindow(id);
  },

  moveWindow: (id, position) => {
    set((state) => ({
      windows: state.windows.map((w) =>
        w.id === id ? { ...w, x: position.x, y: position.y } : w,
      ),
    }));
  },

  resizeWindow: (id, size) => {
    set((state) => ({
      windows: state.windows.map((w) =>
        w.id === id
          ? {
              ...w,
              width: clamp(size.width, MIN_WINDOW_SIZE.width, 4096),
              height: clamp(size.height, MIN_WINDOW_SIZE.height, 4096),
            }
          : w,
      ),
    }));
  },
}));
