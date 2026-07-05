export type VMState = "running" | "paused" | "stopped";

export interface VM {
  id: string;
  user_id: string;
  host_id: string;
  name: string;
  state: VMState;
  cpu_cores: number;
  ram_gb: number;
  storage_gb: number;
  created_at: string;
  updated_at: string;
}

export interface LaunchVMRequest {
  name: string;
  cpu_cores: number;
  ram_gb: number;
  storage_gb: number;
}
