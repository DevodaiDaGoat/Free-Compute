import express from 'express';
import helmet from 'helmet';
import { creditRouter } from './routes/credits';
import { transactionRouter } from './routes/transactions';

const app = express();
const PORT = process.env.PORT || 3002;

app.use(helmet());
app.use(express.json({ limit: '100kb' }));

app.get('/health', (_req, res) => {
  res.json({ status: 'ok' });
});

// Internal API — called by gateway, not exposed to public
app.use('/credits', creditRouter);
app.use('/transactions', transactionRouter);

// Global error handler
app.use((err: Error, _req: express.Request, res: express.Response, _next: express.NextFunction) => {
  console.error('Unhandled error:', err.message);
  res.status(500).json({ error: 'Internal server error' });
});

app.listen(PORT, () => {
  console.log(`billing-service listening on port ${PORT}`);
});

export default app;
