"use client";

import { useEffect, useState } from "react";

import { formatDate, formatTime } from "@/lib/utils";

export default function Clock() {
  const [now, setNow] = useState<Date | null>(null);

  useEffect(() => {
    setNow(new Date());
    const interval = setInterval(() => setNow(new Date()), 1000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="flex flex-col items-end px-2 text-right leading-tight text-white">
      <span className="text-sm tabular-nums">{now ? formatTime(now) : "--:--:--"}</span>
      <span className="text-[10px] text-white/50">{now ? formatDate(now) : ""}</span>
    </div>
  );
}
