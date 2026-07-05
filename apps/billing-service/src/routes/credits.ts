import { Router } from 'express';
import { getBalance, deductCredits, addCredits } from '../controllers/creditController';

export const creditRouter = Router();

creditRouter.get('/:user_id', getBalance);
creditRouter.post('/deduct', deductCredits);
creditRouter.post('/add', addCredits);
