'use client';

import { useCallback, useEffect, useState } from 'react';
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
    return <Desktop />;
  }

  return (
    <BootSequence
      phase={phase}
      onPhaseChange={handlePhaseChange}
      onBootComplete={handleBootComplete}
    />
  );
}
