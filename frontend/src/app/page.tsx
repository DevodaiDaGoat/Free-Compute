"use client";

import dynamic from "next/dynamic";

import BootSequence from "@/components/boot/BootSequence";
import { useDesktopStore } from "@/stores/desktopStore";

const Desktop = dynamic(() => import("@/components/desktop/Desktop"), {
  ssr: false,
});

export default function Home() {
  const isLoggedIn = useDesktopStore((s) => s.isLoggedIn);

  return isLoggedIn ? <Desktop /> : <BootSequence />;
}
