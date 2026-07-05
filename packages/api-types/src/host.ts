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
}

export interface HostMetrics {
  hostId: string;
  cpuUsagePercent: number;
  ramUsagePercent: number;
  gpuUsagePercent?: number;
  storageUsedGb: number;
  activeVMs: number;
  timestamp: string;
}
