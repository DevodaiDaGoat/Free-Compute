import type { ResourceClass, SessionMode, SessionType, StreamPreset } from './remote';

export interface QueueEntry {
  id: string;
  userId: string;
  position: number;
  joinedAt: string;
  estimatedWaitSeconds: number;
  sessionType?: SessionType;
  sessionMode?: SessionMode;
  resourceClass?: ResourceClass;
  gpuPreferred?: boolean;
  latencyBudgetMs?: number;
  updatedAt: string;
}

export interface QueueStatusResponse {
  inQueue: boolean;
  entry?: QueueEntry;
  totalInQueue: number;
}

export interface JoinQueueRequest {
  cpuCores: number;
  ramGb: number;
  storageGb: number;
  gpuVramGb?: number;
  sessionType?: SessionType;
  sessionMode?: SessionMode;
  resourceClass?: ResourceClass;
  streamPreset?: StreamPreset;
  gpuPreferred?: boolean;
  gpuRequired?: boolean;
  latencyBudgetMs?: number;
  region?: string;
}
