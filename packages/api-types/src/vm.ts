export type VMState = 'running' | 'paused' | 'stopped' | 'starting' | 'error';

export interface VM {
  id: string;
  userId: string;
  hostId: string;
  name: string;
  state: VMState;
  cpuCores: number;
  ramGb: number;
  storageGb: number;
  gpuVramGb?: number; // Optional GPU allocation
  createdAt: string;
  updatedAt: string;
}

export interface LaunchVMRequest {
  name: string;
  cpuCores: number;
  ramGb: number;
  storageGb: number;
  gpuVramGb?: number;
  region?: string; // Preferred region
}

export interface LaunchVMResponse {
  vm: VM;
  estimatedReadySeconds: number;
}
