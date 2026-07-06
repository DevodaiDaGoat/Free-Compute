export type ProxyProtocol =
  | 'http'
  | 'https'
  | 'tcp'
  | 'udp'
  | 'websocket'
  | 'webrtc'
  | 'p2p'
  | 'ssh';

export type ProxyClientKind =
  | 'browser'
  | 'webos-app'
  | 'native-client'
  | 'host-agent'
  | 'edge-worker';

export type ProxyRouteMode = 'edge-relay' | 'direct-p2p' | 'host-tunnel';

export type ProxyRouteState =
  | 'requested'
  | 'authorizing'
  | 'connecting'
  | 'active'
  | 'draining'
  | 'expired'
  | 'failed'
  | 'closed';

export type ProxyTransport =
  | 'http-connect'
  | 'websocket'
  | 'webrtc-data-channel'
  | 'webtransport'
  | 'quic'
  | 'tcp'
  | 'udp';

export type TlsTerminationMode = 'edge' | 'origin' | 'passthrough' | 'none';

export interface ProxyTarget {
  hostId?: string;
  vmId?: string;
  deviceId?: string;
  serviceName?: string;
  hostname?: string;
  ipAddress?: string;
  port?: number;
  path?: string;
}

export interface ProxyAuthPolicy {
  required: boolean;
  bearerToken?: string;
  temporaryAccessLinkId?: string;
  allowedUserIds?: string[];
  allowedOrigins?: string[];
  deviceVerificationRequired?: boolean;
}

export interface ProxyRouteRequest {
  protocol: ProxyProtocol;
  clientKind: ProxyClientKind;
  mode: ProxyRouteMode;
  target: ProxyTarget;
  tlsTermination: TlsTerminationMode;
  lowLatency: boolean;
  allowUdpFallback?: boolean;
  requestedRegion?: string;
  auth?: ProxyAuthPolicy;
  metadata?: Record<string, string>;
}

export interface ProxyIngressEndpoint {
  id: string;
  region: string;
  protocol: ProxyProtocol;
  transport: ProxyTransport;
  hostname?: string;
  url?: string;
  port?: number;
  supportsWebTransport?: boolean;
  supportsBunnyCdnAcceleration?: boolean;
}

export interface ProxyRoute {
  id: string;
  userId: string;
  sessionId?: string;
  protocol: ProxyProtocol;
  clientKind: ProxyClientKind;
  mode: ProxyRouteMode;
  state: ProxyRouteState;
  target: ProxyTarget;
  ingress: ProxyIngressEndpoint;
  tlsTermination: TlsTerminationMode;
  createdAt: string;
  updatedAt: string;
  expiresAt?: string;
  closedAt?: string;
}

export interface ProxyPolicy {
  allowedProtocols: ProxyProtocol[];
  allowedClientKinds: ProxyClientKind[];
  maxIdleSeconds: number;
  maxSessionSeconds: number;
  requireAuditLog: boolean;
  requireUserApprovalForControl: boolean;
  blockPrivateTargetsByDefault: boolean;
}

export interface ProxyMetrics {
  routeId: string;
  activeConnections: number;
  bytesIn: number;
  bytesOut: number;
  packetsLost?: number;
  rttMs?: number;
  jitterMs?: number;
  throughputMbps?: number;
  sampledAt: string;
}

export interface CreateProxyRouteResponse {
  route: ProxyRoute;
  connectionToken: string;
  expiresAt: string;
}

export interface SshProxyProfile {
  routeId: string;
  username: string;
  hostKeyFingerprint?: string;
  terminalMode: 'browser-terminal' | 'webos-terminal' | 'native-ssh';
  agentForwardingAllowed: boolean;
  fileTransferAllowed: boolean;
}

export interface ProxyClientPathSet {
  proxyPath?: string;
  websocketTunnelPath?: string;
  connectPath?: string;
  signalingPath?: string;
  rawTcpListener?: boolean;
  rawUdpListener?: boolean;
}

export interface UniversalProxyCapabilities {
  gateway: 'freecompute-universal-proxy';
  protocols: ProxyProtocol[];
  transports: ProxyTransport[];
  clientPaths: Record<ProxyClientKind, ProxyClientPathSet>;
  routeModes: ProxyRouteMode[];
  bunnyCdn: {
    cacheable: string[];
    bypassCache: string[];
    supportsAcceleration: boolean;
  };
}
