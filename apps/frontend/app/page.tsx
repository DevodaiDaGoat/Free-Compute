'use client';

import Link from 'next/link';
import { useState, useEffect } from 'react';

export default function Home() {
  const [gatewayOnline, setGatewayOnline] = useState<boolean | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const resp = await fetch('http://localhost:8080/healthz');
        if (!cancelled) setGatewayOnline(resp.ok);
      } catch {
        if (!cancelled) setGatewayOnline(false);
      }
    })();
    return () => { cancelled = true; };
  }, []);

  return (
    <main style={{
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      minHeight: '100vh',
      padding: '24px',
      textAlign: 'center',
      background: 'var(--bg)',
      color: 'var(--text)',
    }}>
      <div style={{ maxWidth: 640 }}>
        <h1 style={{
          margin: 0,
          fontSize: 'clamp(40px, 7vw, 72px)',
          lineHeight: 1.05,
          letterSpacing: '-0.02em',
          color: 'var(--text)',
        }}>
          FreeCompute
        </h1>
        <p style={{
          margin: '20px 0 0',
          fontSize: 18,
          lineHeight: 1.55,
          color: 'var(--muted)',
        }}>
          Community-powered cloud computing. Run remote desktops, development
          environments, and game streams from shared hosts.
        </p>
        <Link
          href="/webos"
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            marginTop: 36,
            minHeight: 56,
            padding: '0 36px',
            border: '1px solid var(--line)',
            borderRadius: 10,
            background: 'var(--panel)',
            color: 'var(--text)',
            fontSize: 16,
            fontWeight: 750,
            textDecoration: 'none',
            boxShadow: 'var(--shadow)',
          }}
        >
          Launch WebOS
        </Link>
        <Link
          href="/connect"
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            marginTop: 16,
            minHeight: 56,
            padding: '0 36px',
            border: '1px solid var(--line)',
            borderRadius: 10,
            background: 'transparent',
            color: 'var(--text)',
            fontSize: 16,
            fontWeight: 750,
            textDecoration: 'none',
            boxShadow: 'none',
            marginLeft: 12,
          }}
        >
          Connection Space
        </Link>
      </div>
      <footer style={{
        position: 'fixed',
        bottom: 24,
        display: 'inline-flex',
        alignItems: 'center',
        gap: 8,
        fontSize: 13,
        fontWeight: 750,
        color: 'var(--muted)',
      }}>
        <span style={{
          width: 8,
          height: 8,
          borderRadius: '50%',
          background: gatewayOnline === true ? '#238636' : gatewayOnline === false ? '#f85149' : '#8b949e',
          display: 'inline-block',
        }} />
        <span>{gatewayOnline === true ? 'Gateway online' : gatewayOnline === false ? 'Gateway offline' : 'Gateway status: checking…'}</span>
      </footer>
    </main>
  );
}
