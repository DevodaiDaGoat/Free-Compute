import { create } from "zustand";

import { appRegistry, getApp } from "@/components/apps/registry";
import type { App } from "@/lib/types";
import { useWindowStore } from "@/stores/windowStore";

interface AppState {
  registry: App[];
  launchApp: (appId: string) => string | null;
  getAppById: (appId: string) => App | undefined;
}

export const useAppStore = create<AppState>(() => ({
  registry: appRegistry,

  getAppById: (appId) => getApp(appId),

  launchApp: (appId) => {
    const app = getApp(appId);
    if (!app) return null;
    return useWindowStore.getState().openWindow({
      app: app.id,
      title: app.title,
      width: app.defaultWidth,
      height: app.defaultHeight,
      resizable: app.resizable,
      closable: app.closable,
    });
  },
}));
