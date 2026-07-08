'use client';

import { useEffect, useRef } from 'react';

const PREWARM_TTL = 10_000;

export function usePrewarmConnection(url: string, active = true) {
  const linkRef = useRef<HTMLLinkElement | null>(null);

  useEffect(() => {
    if (!active || !url) return;

    const link = document.createElement('link');
    link.rel = 'preconnect';
    link.href = url;
    link.crossOrigin = 'anonymous';
    document.head.appendChild(link);
    linkRef.current = link;

    return () => {
      setTimeout(() => {
        if (linkRef.current && linkRef.current.parentNode) {
          linkRef.current.parentNode.removeChild(linkRef.current);
        }
        linkRef.current = null;
      }, PREWARM_TTL);
    };
  }, [url, active]);
}
