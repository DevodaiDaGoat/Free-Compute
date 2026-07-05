import type { ComponentType } from "react";

export type Theme = "dark" | "light";

export type BootPhase = "bios" | "loading" | "login" | "desktop";

export interface DesktopWindow {
  id: string;
  title: string;
  app: string;
  x: number;
  y: number;
  width: number;
  height: number;
  zIndex: number;
  minimized: boolean;
  maximized: boolean;
  focused: boolean;
  closable: boolean;
  resizable: boolean;
}

export interface AppWindowProps {
  window: DesktopWindow;
}

export interface App {
  id: string;
  title: string;
  icon: string;
  component: ComponentType<AppWindowProps>;
  defaultWidth: number;
  defaultHeight: number;
  resizable: boolean;
  closable: boolean;
}

export type NotificationVariant = "info" | "success" | "warning" | "error";

export interface Notification {
  id: string;
  title: string;
  message: string;
  variant: NotificationVariant;
  createdAt: number;
}

export interface ContextMenuItem {
  id: string;
  label: string;
  onClick: () => void;
  disabled?: boolean;
  separatorAfter?: boolean;
}

export interface ContextMenuState {
  x: number;
  y: number;
  items: ContextMenuItem[];
}

export interface Point {
  x: number;
  y: number;
}

export interface Size {
  width: number;
  height: number;
}
