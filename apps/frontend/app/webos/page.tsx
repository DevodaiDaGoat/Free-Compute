'use client';

import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import BootSequence from './boot/BootSequence';
import Desktop from './desktop/Desktop';

type Phase = 'bios' | 'loading' | 'login' | 'desktop';

export default function WebOS() {
  const [phase, setPhase] = useState<Phase>('bios');

  useEffect(() => {
    document.title = 'FreeCompute WebOS';
  }, []);

  const handlePhaseChange = useCallback((p: Phase) => {
    setPhase(p);
  }, []);

  const handleBootComplete = useCallback(() => {
    setPhase('desktop');
  }, []);

  if (phase === 'desktop') {
    return (
      <>
        <Desktop />
        <nav style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '8px 16px',
          background: 'rgba(17, 17, 40, 0.9)',
          borderBottom: '1px solid #2a2a4a',
          zIndex: 9999,
          backdropFilter: 'blur(8px)',
        }}>
          <div style={{ display: 'flex', gap: 16 }}>
            <Link href="/" style={{ color: '#18e2ff', textDecoration: 'none', fontSize: 14, fontWeight: 600 }}>Back to Home</Link>
            <Link href="/connect" style={{ color: '#18e2ff', textDecoration: 'none', fontSize: 14, fontWeight: 600 }}>Connection Space</Link>
          </div>
          <Link href="/gateway" style={{ color: '#1abc9c', textDecoration: 'none', fontSize: 14, fontWeight: 600 }}>Gateway Console</Link>
        </nav>
      </>
    );
  }

  return (
    <BootSequence
      phase={phase}
      onPhaseChange={handlePhaseChange}
      onBootComplete={handleBootComplete}
    />
  );
}
