import type { ResourceClass, VideoCodec } from './remote';

export type EncoderSupport = 'hardware' | 'software' | 'unsupported';

export interface HostGpu {
  model: string;
  vendor?: 'nvidia' | 'amd' | 'intel' | 'apple' | 'other';
  vramGb: number;
  driverVersion?: string;
  encoderSupport: Partial<Record<VideoCodec, EncoderSupport>>;
  maxConcurrentStreams?: number;
}

export interface HostNetworkProfile {
  publicIpv4?: string;
  publicIpv6?: string;
  region: string;
  availabilityZone?: string;
  uplinkMbps: number;
  downlinkMbps: number;
  p50LatencyMs?: number;
  p95LatencyMs?: number;
  supportsUdp: boolean;
  supportsP2p: boolean;
}

export interface HostCapabilities {
  resourceClasses: ResourceClass[];
  gpuScheduling: boolean;
  hardwareAcceleration: boolean;
  controllerPassthrough: boolean;
  audioForwarding: boolean;
  fileTransfer: boolean;
  remoteSupport: boolean;
  webRtc: boolean;
  tcpProxy: boolean;
  udpProxy: boolean;
  sshProxy: boolean;
}

export interface Host {
  id: string;
  name: string;
  region: string;
  cpuCores: number;
  ramGb: number;
  gpuVramGb: number;
  storageGb: number; // Added: was missing from original schema
  online: boolean;
  lastHeartbeat: string;
  createdAt: string;
  updatedAt: string; // Added: was missing from original schema
  resourceClasses?: ResourceClass[];
  gpus?: HostGpu[];
  network?: HostNetworkProfile;
  capabilities?: HostCapabilities;
}

export interface HostMetrics {
  hostId: string;
  cpuUsagePercent: number;
  ramUsagePercent: number;
  gpuUsagePercent?: number;
  gpuVramUsedGb?: number;
  storageUsedGb: number;
  activeVMs: number;
  activeStreams?: number;
  activeProxyRoutes?: number;
  encoderUsagePercent?: number;
  networkTxMbps?: number;
  networkRxMbps?: number;
  p95LatencyMs?: number;
  timestamp: string;
}
