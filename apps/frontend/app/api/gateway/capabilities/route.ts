import { NextResponse } from 'next/server';

import type { UniversalProxyCapabilities } from '@free-compute/api-types';

export const dynamic = 'force-dynamic';

const gatewayUrl = process.env.FREECOMPUTE_GATEWAY_URL ?? 'http://127.0.0.1:8080';

const fallbackCapabilities: UniversalProxyCapabilities = {
  gateway: 'freecompute-universal-proxy',
  protocols: ['http', 'https', 'websocket', 'tcp', 'udp', 'ssh', 'webrtc', 'p2p'],
  transports: ['http-connect', 'websocket', 'webrtc-data-channel', 'tcp', 'udp'],
  clientPaths: {
    browser: {
      proxyPath: '/proxy/{routeID}/{path}',
      websocketTunnelPath: '/ws/{routeID}',
      signalingPath: '/signal/{routeID}/rooms/{roomID}',
    },
    'webos-app': {
      proxyPath: '/proxy/{routeID}/{path}',
      connectPath: '/connect/{routeID}',
      websocketTunnelPath: '/ws/{routeID}',
      signalingPath: '/signal/{routeID}/rooms/{roomID}',
      rawTcpListener: true,
      rawUdpListener: true,
    },
    'native-client': {
      connectPath: '/connect/{routeID}',
      signalingPath: '/signal/{routeID}/rooms/{roomID}',
      rawTcpListener: true,
      rawUdpListener: true,
    },
    'host-agent': {
      connectPath: '/agent/{routeID}',
    },
    'edge-worker': {
      proxyPath: '/proxy/{routeID}/{path}',
      signalingPath: '/signal/{routeID}/rooms/{roomID}',
    },
  },
  routeModes: ['edge-relay', 'direct-p2p', 'host-tunnel'],
  bunnyCdn: {
    cacheable: ['static frontend assets', 'immutable WebOS assets', 'relay discovery documents'],
    bypassCache: ['/proxy/*', '/ws/*', '/connect/*', '/agent/*', '/signal/*'],
    supportsAcceleration: true,
  },
};

export async function GET() {
  try {
    const response = await fetch(new URL('/capabilities', gatewayUrl), {
      cache: 'no-store',
      signal: AbortSignal.timeout(1500),
    });

    if (!response.ok) {
      throw new Error(`Gateway returned ${response.status}`);
    }

    const capabilities = (await response.json()) as UniversalProxyCapabilities;
    return NextResponse.json({ live: true, gatewayUrl, capabilities });
  } catch (error) {
    return NextResponse.json({
      live: false,
      gatewayUrl,
      capabilities: fallbackCapabilities,
      error: error instanceof Error ? error.message : 'Gateway unavailable',
    });
  }
}
