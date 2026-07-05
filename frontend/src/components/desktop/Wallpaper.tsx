"use client";

import { useDesktopStore } from "@/stores/desktopStore";

export default function Wallpaper() {
  const wallpaper = useDesktopStore((s) => s.wallpaper);

  return (
    <div
      className="absolute inset-0 -z-10 bg-cover bg-center"
      style={{ background: wallpaper }}
      aria-hidden
    />
  );
}
