export interface CreditBalance {
  userId: string;
  balance: number;
  lastUpdated: string;
}

export interface Transaction {
  id: string;
  userId: string;
  amount: number; // positive = credit, negative = debit
  type: 'purchase' | 'usage' | 'reward' | 'refund';
  description: string;
  createdAt: string;
}

export interface PurchaseCreditsRequest {
  amount: number;
  paymentMethodId: string;
}

export interface PurchaseCreditsResponse {
  transaction: Transaction;
  newBalance: number;
}
