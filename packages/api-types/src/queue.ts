export interface QueueEntry {
  id: string;
  user_id: string;
  position: number;
  joined_at: string;
  estimated_wait_seconds: number;
  updated_at: string;
}

export interface QueueStatus {
  position: number;
  estimated_wait_seconds: number;
  total_in_queue: number;
}
