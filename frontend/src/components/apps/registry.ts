import Browser from "@/components/apps/browser/Browser";
import Calculator from "@/components/apps/calculator/Calculator";
import FileManager from "@/components/apps/files/FileManager";
import Settings from "@/components/apps/settings/Settings";
import Terminal from "@/components/apps/terminal/Terminal";
import type { App } from "@/lib/types";

export const appRegistry: App[] = [
  {
    id: "terminal",
    title: "Terminal",
    icon: "⌨️",
    component: Terminal,
    defaultWidth: 640,
    defaultHeight: 400,
    resizable: true,
    closable: true,
  },
  {
    id: "browser",
    title: "Browser",
    icon: "🌐",
    component: Browser,
    defaultWidth: 900,
    defaultHeight: 600,
    resizable: true,
    closable: true,
  },
  {
    id: "files",
    title: "Files",
    icon: "📁",
    component: FileManager,
    defaultWidth: 720,
    defaultHeight: 460,
    resizable: true,
    closable: true,
  },
  {
    id: "settings",
    title: "Settings",
    icon: "⚙️",
    component: Settings,
    defaultWidth: 560,
    defaultHeight: 480,
    resizable: true,
    closable: true,
  },
  {
    id: "calculator",
    title: "Calculator",
    icon: "🧮",
    component: Calculator,
    defaultWidth: 320,
    defaultHeight: 460,
    resizable: false,
    closable: true,
  },
];

const registryMap = new Map<string, App>(
  appRegistry.map((app) => [app.id, app]),
);

export function getApp(appId: string): App | undefined {
  return registryMap.get(appId);
}
