"use client";

import { useCallback } from "react";

import BIOSScreen from "@/components/boot/BIOSScreen";
import LoadingScreen from "@/components/boot/LoadingScreen";
import LoginScreen from "@/components/boot/LoginScreen";
import { useDesktopStore } from "@/stores/desktopStore";

export default function BootSequence() {
  const bootPhase = useDesktopStore((s) => s.bootPhase);
  const setBootPhase = useDesktopStore((s) => s.setBootPhase);

  const goToLoading = useCallback(() => setBootPhase("loading"), [setBootPhase]);
  const goToLogin = useCallback(() => setBootPhase("login"), [setBootPhase]);

  return (
    <div className="h-screen w-screen overflow-hidden">
      {bootPhase === "bios" && <BIOSScreen onComplete={goToLoading} />}
      {bootPhase === "loading" && <LoadingScreen onComplete={goToLogin} />}
      {bootPhase === "login" && <LoginScreen />}
    </div>
  );
}
