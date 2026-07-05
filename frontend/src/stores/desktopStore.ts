import { create } from "zustand";

import { DEFAULT_WALLPAPER } from "@/lib/constants";
import type {
  BootPhase,
  ContextMenuState,
  Notification,
  NotificationVariant,
  Theme,
} from "@/lib/types";
import { generateId } from "@/lib/utils";

interface DesktopState {
  theme: Theme;
  wallpaper: string;
  isLoggedIn: boolean;
  bootPhase: BootPhase;
  notifications: Notification[];
  contextMenu: ContextMenuState | null;
  setTheme: (theme: Theme) => void;
  toggleTheme: () => void;
  setWallpaper: (wallpaper: string) => void;
  setBootPhase: (phase: BootPhase) => void;
  login: (username: string) => void;
  logout: () => void;
  pushNotification: (
    notification: Omit<Notification, "id" | "createdAt">,
  ) => void;
  dismissNotification: (id: string) => void;
  openContextMenu: (menu: ContextMenuState) => void;
  closeContextMenu: () => void;
}

export const useDesktopStore = create<DesktopState>((set) => ({
  theme: "dark",
  wallpaper: DEFAULT_WALLPAPER,
  isLoggedIn: false,
  bootPhase: "bios",
  notifications: [],
  contextMenu: null,

  setTheme: (theme) => set({ theme }),
  toggleTheme: () =>
    set((state) => ({ theme: state.theme === "dark" ? "light" : "dark" })),
  setWallpaper: (wallpaper) => set({ wallpaper }),
  setBootPhase: (bootPhase) => set({ bootPhase }),

  login: () => set({ isLoggedIn: true, bootPhase: "desktop" }),
  logout: () => set({ isLoggedIn: false, bootPhase: "login" }),

  pushNotification: (notification) =>
    set((state) => ({
      notifications: [
        ...state.notifications,
        {
          ...notification,
          id: generateId("notif"),
          createdAt: Date.now(),
        },
      ],
    })),
  dismissNotification: (id) =>
    set((state) => ({
      notifications: state.notifications.filter((n) => n.id !== id),
    })),

  openContextMenu: (contextMenu) => set({ contextMenu }),
  closeContextMenu: () => set({ contextMenu: null }),
}));

export type { NotificationVariant };
