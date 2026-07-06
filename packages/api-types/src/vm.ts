import type { ResourceClass, SessionMode, StreamPreset } from './remote';

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
  resourceClass?: ResourceClass;
  preferredSessionMode?: SessionMode;
  streamPreset?: StreamPreset;
  gpuDedicated?: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface LaunchVMRequest {
  name: string;
  cpuCores: number;
  ramGb: number;
  storageGb: number;
  gpuVramGb?: number;
  resourceClass?: ResourceClass;
  preferredSessionMode?: SessionMode;
  streamPreset?: StreamPreset;
  gpuRequired?: boolean;
  gpuPreferred?: boolean;
  latencyBudgetMs?: number;
  region?: string; // Preferred region
}

export interface LaunchVMResponse {
  vm: VM;
  estimatedReadySeconds: number;
}
