import { Router } from 'express';
import { listTransactions, createTransaction } from '../controllers/transactionController';

export const transactionRouter = Router();

transactionRouter.get('/:user_id', listTransactions);
transactionRouter.post('/', createTransaction);
