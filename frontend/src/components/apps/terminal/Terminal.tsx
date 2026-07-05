"use client";

import { useState } from "react";

import type { AppWindowProps } from "@/lib/types";

interface Line {
  id: number;
  text: string;
}

const BANNER = "FreeCompute WebOS Terminal — pre-alpha (stub)";

export default function Terminal(_props: AppWindowProps) {
  void _props;
  const [lines, setLines] = useState<Line[]>([
    { id: 0, text: BANNER },
    { id: 1, text: 'Type "help" to list available commands.' },
  ]);
  const [input, setInput] = useState("");

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    const command = input.trim();
    const next: Line[] = [{ id: Date.now(), text: `guest@free-compute:~$ ${command}` }];

    if (command === "help") {
      next.push({ id: Date.now() + 1, text: "Available: help, clear, whoami, echo <text>" });
    } else if (command === "clear") {
      setLines([]);
      setInput("");
      return;
    } else if (command === "whoami") {
      next.push({ id: Date.now() + 1, text: "guest" });
    } else if (command.startsWith("echo ")) {
      next.push({ id: Date.now() + 1, text: command.slice(5) });
    } else if (command.length > 0) {
      next.push({ id: Date.now() + 1, text: `command not found: ${command}` });
    }

    setLines((prev) => [...prev, ...next]);
    setInput("");
  };

  return (
    <div className="flex h-full flex-col bg-black/70 p-3 font-mono text-sm text-green-400">
      <div className="flex-1 overflow-auto">
        {lines.map((line) => (
          <div key={line.id} className="whitespace-pre-wrap break-words">
            {line.text}
          </div>
        ))}
      </div>
      <form onSubmit={handleSubmit} className="mt-2 flex items-center gap-2">
        <span className="shrink-0 text-[var(--accent)]">guest@free-compute:~$</span>
        <input
          value={input}
          onChange={(event) => setInput(event.target.value)}
          className="flex-1 bg-transparent text-green-400 outline-none"
          autoFocus
          spellCheck={false}
        />
      </form>
    </div>
  );
}
