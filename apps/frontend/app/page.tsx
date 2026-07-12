'use client';

import Link from 'next/link';
import { useState, useEffect } from 'react';
import {
  Monitor, Cpu, Zap, Shield, Globe, Play,
  GitBranch, Server, Gamepad2, Wifi, ChevronRight, Star, ArrowRight,
} from 'lucide-react';
import { getGatewayUrl } from './webos/boot/BootSequence';

function useGatewayStatus() {
  const [status, setStatus] = useState<'unknown' | 'online' | 'offline'>('unknown');
  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();
    fetch(`${getGatewayUrl()}/healthz`, { signal: controller.signal })
      .then((r) => { if (!cancelled) setStatus(r.ok ? 'online' : 'offline'); })
      .catch(() => { if (!cancelled) setStatus('offline'); });
    return () => { cancelled = true; controller.abort(); };
  }, []);
  return status;
}

function StatusDot({ status }: { status: 'unknown' | 'online' | 'offline' }) {
  const color = status === 'online' ? '#3fb950' : status === 'offline' ? '#f85149' : '#6e7681';
  const bg = status === 'online' ? 'rgba(63,185,80,0.1)' : status === 'offline' ? 'rgba(248,81,73,0.1)' : 'rgba(110,118,129,0.1)';
  const border = status === 'online' ? 'rgba(63,185,80,0.3)' : status === 'offline' ? 'rgba(248,81,73,0.3)' : 'rgba(110,118,129,0.2)';
  const label = status === 'online' ? 'Gateway online' : status === 'offline' ? 'Gateway offline' : 'Checking...';
  return (
    <span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color, padding: '5px 10px', borderRadius: 6, background: bg, border: `1px solid ${border}`, fontWeight: 600 }}>
      <span style={{ width: 6, height: 6, borderRadius: '50%', background: 'currentColor', display: 'inline-block' }} />
      {label}
    </span>
  );
}

const FEATURES = [
  {
    icon: <Monitor size={20} />,
    title: 'Browser WebOS',
    desc: 'Full desktop environment — window manager, terminal, file manager, and apps — entirely in the browser with no installation.',
    accent: '#58a6ff',
  },
  {
    icon: <Zap size={20} />,
    title: 'Galaxy Scheduler',
    desc: 'Selects the best host by region, latency, CPU load, RAM, and GPU availability in real time.',
    accent: '#3fb950',
  },
  {
    icon: <Gamepad2 size={20} />,
    title: 'Gaming Support',
    desc: 'H.265 / AV1 GPU streaming, adaptive bitrate, controller passthrough, sub-20ms latency target.',
    accent: '#d2a8ff',
  },
  {
    icon: <Shield size={20} />,
    title: 'Secure by Default',
    desc: 'JWT auth, TLS, session isolation, sandboxed VMs, audit logging, role-based access control.',
    accent: '#ffa657',
  },
  {
    icon: <Cpu size={20} />,
    title: 'Donate Compute',
    desc: 'Contribute idle CPU, RAM, GPU, and bandwidth. Earn community credits, help others run desktops.',
    accent: '#79c0ff',
  },
  {
    icon: <Wifi size={20} />,
    title: 'WebRTC Streaming',
    desc: 'GCC-style adaptive bitrate engine with codec switching (VP8→VP9→H.264→H.265) based on live network quality.',
    accent: '#56d364',
  },
];

const STATS = [
  { label: 'Gateway Protocols', value: '7' },
  { label: 'Transport Layers', value: '5' },
  { label: 'Session Types', value: '4' },
  { label: 'License', value: 'MIT' },
];

const STEPS = [
  { n: '01', title: 'Open the Browser', body: 'No installation. Visit FreeCompute and launch a session directly from your browser.' },
  { n: '02', title: 'Galaxy Schedules', body: 'The scheduler picks the lowest-latency, highest-capacity host for your session type and region.' },
  { n: '03', title: 'Stream Starts', body: 'WebRTC or WebTransport stream starts within seconds. Full desktop, terminal, or game stream.' },
];

export default function Home() {
  const gwStatus = useGatewayStatus();

  return (
    <div style={{ minHeight: '100vh', background: '#0d1117', color: '#c9d1d9', fontFamily: 'Inter, ui-sans-serif, system-ui, sans-serif', overflowX: 'hidden' }}>

      {/* Ambient glow layer */}
      <div aria-hidden style={{ position: 'fixed', inset: 0, pointerEvents: 'none', zIndex: 0,
        background: 'radial-gradient(ellipse 70% 50% at 50% -5%, rgba(88,166,255,0.09) 0%, transparent 55%), radial-gradient(ellipse 50% 35% at 85% 90%, rgba(35,134,54,0.06) 0%, transparent 50%)',
      }} />

      {/* Nav */}
      <header style={{ position: 'sticky', top: 0, zIndex: 100, borderBottom: '1px solid rgba(33,38,45,0.8)', backdropFilter: 'blur(12px)', background: 'rgba(13,17,23,0.85)' }}>
        <nav style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 clamp(16px, 4vw, 40px)', height: 58, maxWidth: 1200, margin: '0 auto', gap: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 9 }}>
            <span style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: 30, height: 30, borderRadius: 8, background: 'rgba(88,166,255,0.15)', border: '1px solid rgba(88,166,255,0.2)' }}>
              <Server size={14} color="#58a6ff" />
            </span>
            <span style={{ fontWeight: 700, fontSize: 15, color: '#e6edf3', letterSpacing: '-0.01em' }}>FreeCompute</span>
            <span style={{ fontSize: 10, padding: '2px 7px', borderRadius: 20, background: 'rgba(88,166,255,0.1)', border: '1px solid rgba(88,166,255,0.2)', color: '#58a6ff', fontWeight: 700, letterSpacing: '0.04em', textTransform: 'uppercase' }}>Pre-Alpha</span>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <StatusDot status={gwStatus} />
            <a href="https://github.com/DevodaiDaGoat/Free-Compute" target="_blank" rel="noreferrer" style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '6px 12px', borderRadius: 6, border: '1px solid #30363d', color: '#8b949e', textDecoration: 'none', fontSize: 13, fontWeight: 600 }}>
              <GitBranch size={13} />
              GitHub
            </a>
            <Link href="/gateway" style={{ padding: '6px 14px', borderRadius: 6, border: '1px solid #30363d', color: '#8b949e', textDecoration: 'none', fontSize: 13, fontWeight: 600 }}>
              Console
            </Link>
          </div>
        </nav>
      </header>

      <main style={{ position: 'relative', zIndex: 1 }}>

        {/* Hero */}
        <section style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', textAlign: 'center', padding: 'clamp(64px, 12vh, 110px) clamp(16px, 5vw, 32px) clamp(48px, 8vh, 80px)' }}>

          <div style={{ display: 'inline-flex', alignItems: 'center', gap: 7, marginBottom: 28, padding: '5px 14px', borderRadius: 20, background: 'rgba(88,166,255,0.08)', border: '1px solid rgba(88,166,255,0.18)', fontSize: 12, color: '#79c0ff', fontWeight: 700, letterSpacing: '0.06em', textTransform: 'uppercase' }}>
            <Star size={11} fill="#79c0ff" />
            Open Source &middot; Community Powered
          </div>

          <h1 style={{ margin: '0 0 22px', fontSize: 'clamp(38px, 7vw, 72px)', fontWeight: 800, lineHeight: 1.04, letterSpacing: '-0.035em', maxWidth: 800, background: 'linear-gradient(160deg, #e6edf3 20%, #8b949e 100%)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent' }}>
            Cloud Desktops<br />Powered by People
          </h1>

          <p style={{ margin: '0 0 42px', fontSize: 'clamp(16px, 2.2vw, 19px)', lineHeight: 1.7, color: '#8b949e', maxWidth: 540 }}>
            Launch secure cloud desktops, dev environments, and game streams directly from your browser — backed by donated hardware and community infrastructure.
          </p>

          <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', justifyContent: 'center' }}>
            <Link href="/webos" style={{ display: 'inline-flex', alignItems: 'center', gap: 9, padding: '13px 30px', borderRadius: 9, background: 'linear-gradient(135deg, #1f6feb, #388bfd)', color: '#fff', fontSize: 15, fontWeight: 700, textDecoration: 'none', letterSpacing: '-0.01em', boxShadow: '0 0 32px rgba(31,111,235,0.3), 0 4px 12px rgba(0,0,0,0.4)' }}>
              <Play size={15} fill="#fff" />
              Launch WebOS
            </Link>
            <Link href="/connect" style={{ display: 'inline-flex', alignItems: 'center', gap: 9, padding: '13px 30px', borderRadius: 9, background: 'rgba(33,38,45,0.6)', border: '1px solid #30363d', color: '#c9d1d9', fontSize: 15, fontWeight: 700, textDecoration: 'none', letterSpacing: '-0.01em' }}>
              <Globe size={15} />
              Connection Space
              <ArrowRight size={14} />
            </Link>
          </div>

          {/* Stats row */}
          <div style={{ display: 'flex', gap: 0, marginTop: 64, borderRadius: 12, overflow: 'hidden', border: '1px solid #21262d', background: '#161b22', flexWrap: 'wrap' }}>
            {STATS.map((s, i) => (
              <div key={s.label} style={{ padding: '18px 36px', textAlign: 'center', borderRight: i < STATS.length - 1 ? '1px solid #21262d' : 'none', flex: '1 1 120px', minWidth: 100 }}>
                <div style={{ fontSize: 22, fontWeight: 800, color: '#e6edf3', letterSpacing: '-0.02em' }}>{s.value}</div>
                <div style={{ fontSize: 11, color: '#6e7681', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em', marginTop: 4 }}>{s.label}</div>
              </div>
            ))}
          </div>
        </section>

        {/* How it works */}
        <section style={{ padding: 'clamp(48px, 8vh, 80px) clamp(16px, 5vw, 40px)', maxWidth: 1100, margin: '0 auto' }}>
          <div style={{ textAlign: 'center', marginBottom: 48 }}>
            <h2 style={{ margin: '0 0 12px', fontSize: 'clamp(24px, 4vw, 38px)', fontWeight: 800, color: '#e6edf3', letterSpacing: '-0.025em' }}>How it works</h2>
            <p style={{ margin: 0, color: '#8b949e', fontSize: 15 }}>Browser to VM in three steps</p>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: 16 }}>
            {STEPS.map((step) => (
              <div key={step.n} style={{ position: 'relative', padding: '28px 24px', borderRadius: 12, border: '1px solid #21262d', background: '#0d1117', overflow: 'hidden' }}>
                <div style={{ position: 'absolute', top: -8, right: 12, fontSize: 64, fontWeight: 900, color: 'rgba(88,166,255,0.04)', lineHeight: 1, userSelect: 'none', letterSpacing: '-0.04em' }}>{step.n}</div>
                <div style={{ fontSize: 12, fontWeight: 700, color: '#58a6ff', letterSpacing: '0.08em', textTransform: 'uppercase', marginBottom: 12 }}>{step.n}</div>
                <div style={{ fontSize: 16, fontWeight: 700, color: '#e6edf3', marginBottom: 10 }}>{step.title}</div>
                <div style={{ fontSize: 14, color: '#6e7681', lineHeight: 1.65 }}>{step.body}</div>
              </div>
            ))}
          </div>
        </section>

        {/* Features */}
        <section style={{ padding: 'clamp(48px, 8vh, 80px) clamp(16px, 5vw, 40px)', maxWidth: 1100, margin: '0 auto' }}>
          <div style={{ textAlign: 'center', marginBottom: 48 }}>
            <h2 style={{ margin: '0 0 12px', fontSize: 'clamp(24px, 4vw, 38px)', fontWeight: 800, color: '#e6edf3', letterSpacing: '-0.025em' }}>Everything included</h2>
            <p style={{ margin: 0, color: '#8b949e', fontSize: 15 }}>Full stack — from browser to bare metal</p>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: 1, border: '1px solid #21262d', borderRadius: 12, overflow: 'hidden' }}>
            {FEATURES.map((f) => (
              <div key={f.title} style={{ padding: '28px 26px', background: '#0d1117', borderRight: '1px solid #21262d', borderBottom: '1px solid #21262d', display: 'flex', flexDirection: 'column', gap: 14, transition: 'background 0.15s' }}>
                <div style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: 40, height: 40, borderRadius: 10, background: `${f.accent}14`, border: `1px solid ${f.accent}28`, color: f.accent, flexShrink: 0 }}>
                  {f.icon}
                </div>
                <div>
                  <div style={{ fontSize: 15, fontWeight: 700, color: '#e6edf3', marginBottom: 8 }}>{f.title}</div>
                  <div style={{ fontSize: 13, color: '#6e7681', lineHeight: 1.65 }}>{f.desc}</div>
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* Architecture */}
        <section style={{ padding: 'clamp(48px, 8vh, 80px) clamp(16px, 5vw, 40px)', maxWidth: 1100, margin: '0 auto' }}>
          <div style={{ borderRadius: 12, border: '1px solid #21262d', background: '#0d1117', overflow: 'hidden' }}>
            <div style={{ padding: '28px 28px 0' }}>
              <h2 style={{ margin: '0 0 6px', fontSize: 22, fontWeight: 800, color: '#e6edf3', letterSpacing: '-0.02em' }}>Architecture</h2>
              <p style={{ margin: '0 0 24px', color: '#6e7681', fontSize: 14 }}>Cloudflare Tunnel → Go Gateway → WebRTC → Browser</p>
            </div>
            <div style={{ padding: '0 28px 28px', fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace', fontSize: 13, color: '#6e7681', lineHeight: 2, overflowX: 'auto' }}>
              <pre style={{ margin: 0 }}>{`Browser Client
    │  WebRTC / WebSocket / WebTransport
    ▼
Cloudflare Tunnel
    │
API Gateway  (Go · :8080)
    ├─ Auth / JWT
    ├─ Galaxy Scheduler
    ├─ Security Detector
    ├─ Rate Limiter
    │
    ├─── Community Hosts
    │       └─ Host Agent  →  QEMU VM
    └─── Cloud Hosts
             └─ Host Agent  →  QEMU VM`}</pre>
            </div>
          </div>
        </section>

        {/* Contribute CTA */}
        <section style={{ padding: 'clamp(48px, 8vh, 80px) clamp(16px, 5vw, 40px)', maxWidth: 1100, margin: '0 auto' }}>
          <div style={{ borderRadius: 14, border: '1px solid rgba(88,166,255,0.2)', background: 'linear-gradient(135deg, rgba(31,111,235,0.08) 0%, rgba(35,134,54,0.06) 100%)', padding: 'clamp(32px, 5vw, 56px) clamp(24px, 4vw, 48px)', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 24, flexWrap: 'wrap' }}>
            <div style={{ maxWidth: 500 }}>
              <h2 style={{ margin: '0 0 12px', fontSize: 'clamp(20px, 3.5vw, 32px)', fontWeight: 800, color: '#e6edf3', letterSpacing: '-0.025em' }}>Have idle hardware?</h2>
              <p style={{ margin: 0, color: '#8b949e', fontSize: 15, lineHeight: 1.65 }}>Install the Host Agent and contribute your CPU, RAM, GPU, and bandwidth to the network. Earn community credits. Help others get access to compute.</p>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10, flexShrink: 0 }}>
              <a href="https://github.com/DevodaiDaGoat/Free-Compute" target="_blank" rel="noreferrer" style={{ display: 'inline-flex', alignItems: 'center', gap: 8, padding: '12px 24px', borderRadius: 8, background: '#e6edf3', color: '#0d1117', fontSize: 14, fontWeight: 700, textDecoration: 'none', letterSpacing: '-0.01em' }}>
                <GitBranch size={15} />
                Read the Docs
                <ChevronRight size={14} />
              </a>
              <Link href="/connect" style={{ display: 'inline-flex', alignItems: 'center', gap: 8, padding: '12px 24px', borderRadius: 8, border: '1px solid #30363d', color: '#c9d1d9', fontSize: 14, fontWeight: 700, textDecoration: 'none', letterSpacing: '-0.01em', justifyContent: 'center' }}>
                Try it now
              </Link>
            </div>
          </div>
        </section>

      </main>

      {/* Footer */}
      <footer style={{ borderTop: '1px solid #21262d', padding: '28px clamp(16px, 5vw, 40px)' }}>
        <div style={{ maxWidth: 1100, margin: '0 auto', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 16, flexWrap: 'wrap' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <Server size={13} color="#484f58" />
            <span style={{ fontSize: 13, color: '#484f58', fontWeight: 600 }}>FreeCompute</span>
            <span style={{ fontSize: 12, color: '#30363d' }}>&middot;</span>
            <span style={{ fontSize: 12, color: '#484f58' }}>MIT License</span>
          </div>
          <div style={{ display: 'flex', gap: 20, fontSize: 13, color: '#6e7681' }}>
            <span>Built with Go, Next.js 15, React 19, WebRTC</span>
          </div>
        </div>
      </footer>

    </div>
  );
}
