"use client";

import { useEffect, type ReactNode } from "react";

import { useDesktopStore } from "@/stores/desktopStore";

export default function Providers({ children }: { children: ReactNode }) {
  const theme = useDesktopStore((s) => s.theme);

  useEffect(() => {
    const root = document.documentElement;
    root.classList.toggle("theme-light", theme === "light");
  }, [theme]);

  return <>{children}</>;
}
