"use client";

import { useState } from "react";

import { useDesktopStore } from "@/stores/desktopStore";

export default function LoginScreen() {
  const login = useDesktopStore((s) => s.login);
  const [username, setUsername] = useState("guest");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    if (username.trim().length === 0) {
      setError("Please enter a username.");
      return;
    }
    setError(null);
    login(username.trim());
  };

  return (
    <div className="flex h-full w-full items-center justify-center bg-[var(--bg-primary)]">
      <div className="w-full max-w-sm rounded-2xl border border-[var(--window-border)] bg-[var(--bg-secondary)]/80 p-8 backdrop-blur">
        <div className="mb-6 text-center">
          <div className="text-2xl font-bold text-white">
            Free<span className="text-[var(--accent)]">Compute</span>
          </div>
          <p className="mt-1 text-sm text-white/50">Sign in to your desktop</p>
        </div>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <label className="flex flex-col gap-1 text-sm text-white/70">
            Username
            <input
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              className="rounded-lg bg-black/40 px-3 py-2 text-white outline-none focus:ring-1 focus:ring-[var(--accent)]"
              autoComplete="username"
            />
          </label>
          <label className="flex flex-col gap-1 text-sm text-white/70">
            Password
            <input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              className="rounded-lg bg-black/40 px-3 py-2 text-white outline-none focus:ring-1 focus:ring-[var(--accent)]"
              autoComplete="current-password"
              placeholder="(any password)"
            />
          </label>
          {error && <p className="text-sm text-red-400">{error}</p>}
          <button
            type="submit"
            className="mt-2 rounded-lg bg-[var(--accent)] py-2 font-medium text-black transition-opacity hover:opacity-90"
          >
            Sign In
          </button>
        </form>
      </div>
    </div>
  );
}
