export interface QueueEntry {
  id: string;
  userId: string;
  position: number;
  joinedAt: string;
  estimatedWaitSeconds: number;
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
  region?: string;
}
