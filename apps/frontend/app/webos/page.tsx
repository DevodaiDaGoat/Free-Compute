'use client';

import { useCallback, useEffect, useState } from 'react';
import BootSequence from './boot/BootSequence';
import Desktop from './desktop/Desktop';

export default function WebOS() {
  const [phase, setPhase] = useState<'bios' | 'loading' | 'login' | 'desktop'>('bios');

  const onBootComplete = useCallback(() => {
    setPhase('desktop');
  }, []);

  useEffect(() => {
    document.title = 'FreeCompute WebOS';
  }, []);

  if (phase !== 'desktop') {
    return <BootSequence phase={phase} onPhaseChange={setPhase} onBootComplete={onBootComplete} />;
  }

  return <Desktop />;
}
