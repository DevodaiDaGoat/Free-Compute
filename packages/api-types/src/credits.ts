export interface CreditBalance {
  user_id: string;
  balance: number;
  updated_at: string;
}

export interface Transaction {
  id: string;
  user_id: string;
  amount: number;
  type: TransactionType;
  description: string;
  created_at: string;
}

export type TransactionType = "purchase" | "usage" | "reward" | "refund";

export interface PurchaseCreditsRequest {
  amount: number;
  payment_method: string;
}
