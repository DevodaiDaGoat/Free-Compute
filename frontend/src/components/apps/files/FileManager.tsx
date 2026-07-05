"use client";

import { useState } from "react";

import type { AppWindowProps } from "@/lib/types";
import { cn } from "@/lib/utils";

const FOLDERS = ["Home", "Documents", "Downloads", "Pictures", "Projects"];
const FILES = [
  { name: "welcome.txt", size: "2 KB" },
  { name: "notes.md", size: "5 KB" },
  { name: "screenshot.png", size: "482 KB" },
  { name: "budget.csv", size: "12 KB" },
];

export default function FileManager(_props: AppWindowProps) {
  void _props;
  const [active, setActive] = useState("Home");

  return (
    <div className="flex h-full bg-[var(--bg-secondary)] text-white">
      <aside className="w-44 shrink-0 border-r border-[var(--window-border)] p-2">
        <p className="px-2 pb-1 text-xs uppercase tracking-wide text-white/40">
          Places
        </p>
        {FOLDERS.map((folder) => (
          <button
            key={folder}
            type="button"
            onClick={() => setActive(folder)}
            className={cn(
              "flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm",
              active === folder ? "bg-[var(--accent)] text-black" : "hover:bg-white/10",
            )}
          >
            <span>📁</span>
            {folder}
          </button>
        ))}
      </aside>
      <div className="flex-1 overflow-auto p-3">
        <p className="pb-2 text-sm text-white/60">{active}</p>
        <div className="grid grid-cols-[1fr_auto] gap-x-4 gap-y-1 text-sm">
          {FILES.map((file) => (
            <div key={file.name} className="contents">
              <span className="flex items-center gap-2 rounded px-2 py-1 hover:bg-white/10">
                <span>📄</span>
                {file.name}
              </span>
              <span className="px-2 py-1 text-right text-white/40">{file.size}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
