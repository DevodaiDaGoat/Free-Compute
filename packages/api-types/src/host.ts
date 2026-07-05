export interface Host {
  id: string;
  name: string;
  region: string;
  cpu_cores: number;
  ram_gb: number;
  gpu_vram_gb: number;
  online: boolean;
  last_heartbeat: string;
  created_at: string;
}

export interface HostMetrics {
  host_id: string;
  cpu_usage: number;
  ram_usage: number;
  gpu_usage: number;
  disk_usage: number;
  network_rx_bytes: number;
  network_tx_bytes: number;
  timestamp: string;
}

export interface Heartbeat {
  host_id: string;
  metrics: HostMetrics;
  active_vms: number;
}
