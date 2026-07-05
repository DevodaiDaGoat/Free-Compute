export interface Transaction {
  id: string;
  user_id: string;
  type: 'purchase' | 'spend' | 'reward' | 'refund';
  amount: number;
  description: string;
  idempotency_key: string;
  created_at: Date;
}
